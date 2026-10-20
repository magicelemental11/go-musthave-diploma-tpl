# go-musthave-diploma-tpl

Накопительная система лояльности «Гофермарт».

## Запуск

PostgreSQL:

```
docker run --name gophermart-db -d -p 5432:5432 \
  -e POSTGRES_USER=gophermart -e POSTGRES_PASSWORD=gophermart -e POSTGRES_DB=gophermart \
  postgres:17
```

Сервис:

```
go run ./cmd/gophermart \
  -d 'postgres://gophermart:gophermart@localhost:5432/gophermart?sslmode=disable' \
  -r http://localhost:8081
```

Миграции применяются при старте.

## Конфигурация

| Флаг | Переменная окружения | По умолчанию |
|---|---|---|
| `-a` | `RUN_ADDRESS` | `localhost:8080` |
| `-d` | `DATABASE_URI` | обязателен |
| `-r` | `ACCRUAL_SYSTEM_ADDRESS` | обязателен |
| — | `SECRET_KEY` | значение для разработки |

Переменная окружения приоритетнее флага.

## Тесты

```
go vet ./...
go test -race ./...
```
