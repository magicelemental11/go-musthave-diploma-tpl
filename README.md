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

## API

| Метод | Путь | Назначение |
|---|---|---|
| POST | `/api/user/register` | регистрация |
| POST | `/api/user/login` | аутентификация |
| POST | `/api/user/orders` | загрузка номера заказа |
| GET | `/api/user/orders` | список заказов |
| GET | `/api/user/balance` | баланс и сумма списаний |
| POST | `/api/user/balance/withdraw` | списание баллов |
| GET | `/api/user/withdrawals` | история списаний |

Все методы, кроме регистрации и входа, требуют сессионную cookie.

## Тесты

```
go vet ./...
go test -race ./...
```

Тесты хранилища и сквозной сценарий (регистрация, заказ, начисление через
фейковую систему расчёта, списание) выполняются, только если задана
`TEST_DATABASE_URI`; иначе они пропускаются.

```
TEST_DATABASE_URI='postgres://gophermart:gophermart@localhost:5432/gophermart?sslmode=disable' \
  go test -race -p 1 -cover ./...
```
