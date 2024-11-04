package internal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

var testCtx = context.Background()

func fixedIpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "192.168.0.1:1234"

		next.ServeHTTP(w, r)
	})
}

func setupTestRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	})
}

func TestRedisRateLimiter(t *testing.T) {
	os.Setenv("RATE_LIMIT_PER_IP", "100")
	os.Setenv("RATE_LIMIT_PER_TOKEN", "120")
	os.Setenv("RATE_LIMIT_TIME_BLOCK", "3")

	redisClient := setupTestRedis()
	redisClient.FlushDB(testCtx)
	rateLimiter = NewRedisRateLimiter(redisClient)

	r := mux.NewRouter()
	r.Use(fixedIpMiddleware)
	r.Use(rateLimiterMiddleware)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}).Methods("GET")

	testServer := httptest.NewServer(r)
	defer testServer.Close()

	duration, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_TIME_BLOCK"))
	timeBlock := time.Duration(duration) * time.Second

	client := &http.Client{}

	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest("GET", testServer.URL+"/", nil)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	req, _ := http.NewRequest("GET", testServer.URL+"/", nil)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	token := "test-token"
	for i := 0; i < 120; i++ {
		req, _ := http.NewRequest("GET", testServer.URL+"/", nil)
		req.Header.Set("API_KEY", token)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	req, _ = http.NewRequest("GET", testServer.URL+"/", nil)
	req.Header.Set("API_KEY", token)
	resp, err = client.Do(req)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	time.Sleep(timeBlock)

	req, _ = http.NewRequest("GET", testServer.URL+"/", nil)
	resp, err = client.Do(req)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
