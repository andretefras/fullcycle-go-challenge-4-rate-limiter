Antes de iniciar o servidor é necessário criar um container com o Redis executando o comando:

`docker-compose up -d`

Para executar o teste do RedisRateLimiter execute o comando:

`go test ./...`

O rate limiter poderá ser configurado a partir das seguintes variáveis de ambiente:

| Variável de ambiente    | Descrição        | Valor padrão |
|-------------------------|------------------|--------------|
| `RATE_LIMIT_PER_IP`     | Limite por ip    | 1            |
| `RATE_LIMIT_PER_TOKEN`  | Limite por token | 2            |
| `RATE_LIMIT_TIME_BLOCK` | Limite de tempo  | 5            |

Para iniciar o servidor http execute o comando:

`go run cmd/ratelimiter/main.go`

As chamadas para o servidor http poderão ser feitas através do arquivo `api/hello_world.http`

A strategy do RateLimiter poderá ser expandida a partir de novas implementações da interface encontrada em:

`internal/ratelimiter.go:21`