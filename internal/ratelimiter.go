package internal

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

var (
	rateLimiter RateLimiter
	ctx         = context.Background()
)

type RateLimiter interface {
	LimitReached(key string, limit int, duration time.Duration) (bool, error)
}

type RedisRateLimiter struct {
	client *redis.Client
}

func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
	}
}

func (r RedisRateLimiter) LimitReached(key string, limit int, duration time.Duration) (bool, error) {
	count, err := r.client.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, err
	}

	if count >= limit {
		return true, nil
	}

	tx := r.client.TxPipeline()
	tx.Incr(ctx, key)
	tx.Expire(ctx, key, duration)
	_, err = tx.Exec(ctx)

	if err != nil {
		return false, err
	}

	return false, nil
}

func initRateLimiter() {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	rateLimiter = NewRedisRateLimiter(redisClient)
}

func getEnv(key string, defaultValue int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

func rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		token := r.Header.Get("API_KEY")

		limitPerIP, _ := getEnv("RATE_LIMIT_PER_IP", 1)
		limitPerToken, _ := getEnv("RATE_LIMIT_PER_TOKEN", 2)
		timeBlock, _ := getEnv("RATE_LIMIT_TIME_BLOCK", 5)

		duration := time.Duration(timeBlock) * time.Second

		var limit int
		var key string
		if token != "" {
			limit = limitPerToken
			key = fmt.Sprintf("token:%s", token)
		} else {
			limit = limitPerIP
			key = fmt.Sprintf("ip:%s", ip)
		}

		reached, err := rateLimiter.LimitReached(key, limit, duration)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		if reached {
			http.Error(w, "you have reached the maximum number of requests or actions allowed within a certain time frame", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func InitServer() {
	initRateLimiter()

	r := mux.NewRouter()
	r.Use(rateLimiterMiddleware)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, "Hello, World!")
		if err != nil {
			panic(err)
		}
	}).Methods("GET")

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		panic(err)
	}
}
