# Микросервисное приложение на Go

Реализация архитектуры из схемы: **User → Order → Payment → Delivery** с полной event-driven коммуникацией.

## Стек технологий

| Компонент | Технология |
|-----------|-----------|
| Язык | Go 1.23 |
| Межсервисное взаимодействие | gRPC + Protobuf |
| Асинхронные события | Apache Kafka |
| База данных | PostgreSQL 16 (отдельная на сервис) |
| API для клиентов | REST HTTP (API Gateway) |
| Контейнеризация | Docker + Docker Compose |

## Архитектура

```
                    ┌─────────────┐
 Client (curl/app)  │ API Gateway  │  :8080 (HTTP REST)
   ──────────────►  │  gorilla/mux │
                    └──────┬──────┘
                           │ gRPC
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
   │   Order     │  │   Payment   │  │  Delivery   │
   │  Service    │  │   Service   │  │   Service   │
   │  :50051     │  │   :50052    │  │   :50053    │
   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
          │                │                │
          ▼                ▼                ▼
      orders-db       payments-db     deliveries-db
      :5432           :5433           :5434

          └────────────────┼────────────────┘
                           ▼
                    ┌─────────────┐
                    │    Kafka    │  :9092
                    │   Topics:   │
                    │ order.created│
                    │ payment.completed│
                    │ delivery.status.updated│
                    └─────────────┘
```

## Kafka Flow (Event-Driven)

```
User создаёт заказ
    │
    ▼
Order Service → сохраняет в БД → публикует [order.created]
    │
    ▼ (async)
Payment Service подписан на [order.created]
    → проверяет баланс пользователя
    → если достаточно: списывает → публикует [payment.completed: SUCCESS]
    → если мало:        → публикует [payment.completed: INSUFFICIENT_FUNDS]
    │
    ▼ (async)
Order Service подписан на [payment.completed]
    → обновляет статус заказа (PAID / INSUFFICIENT_FUNDS)
    │
    ▼ (async, только при SUCCESS)
Delivery Service подписан на [payment.completed]
    → создаёт доставку → публикует [delivery.status.updated]
    → симулирует прогресс: PENDING → IN_TRANSIT → OUT_FOR_DELIVERY → DELIVERED
    │
    ▼ (async)
Order Service подписан на [delivery.status.updated]
    → обновляет статус доставки в своей БД
```

## Паттерны

- **Outbox Pattern** — атомарная запись заказа + события в одной транзакции (гарантированная доставка)
- **Saga Pattern** — распределённая транзакция: если оплата упала, доставка не создаётся
- **Database per Service** — каждый сервис имеет свою изолированную БД
- **API Gateway** — единая точка входа, проксирует HTTP → gRPC

## Быстрый старт

### Требования
- Docker & Docker Compose
- Go 1.23+ (для локальной разработки)
- `protoc` + плагины (для регенерации proto)

### Запуск

```bash
# Поднять все сервисы
make up

# Проверить статус
make health

# Создать тестовый заказ
make test-create-order

# Посмотреть логи
make logs
```

### Тестирование полного флоу

```bash
# 1. Создаём заказ
ORDER_ID=$(curl -s -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "delivery_address": "Москва, ул. Тверская, 1",
    "items": [
      {"product_id": "prod-1", "quantity": 2, "price": 149.99}
    ]
  }' | jq -r '.order_id')

echo "Order ID: $ORDER_ID"

# 2. Смотрим статус заказа (сразу PENDING)
curl -s http://localhost:8080/api/orders/$ORDER_ID | jq .

# 3. Через 2-3 секунды: статус платежа
curl -s http://localhost:8080/api/orders/$ORDER_ID/payment | jq .

# 4. Статус доставки (появляется после успешной оплаты)
curl -s http://localhost:8080/api/orders/$ORDER_ID/delivery | jq .
```

### Мониторинг Kafka

Открыть в браузере: **http://localhost:8090**

Там видно все топики, сообщения и consumer groups.

## Структура проекта

```
microservices-go/
├── proto/                    # Protobuf определения
│   ├── order.proto
│   ├── payment.proto
│   └── delivery.proto
├── api-gateway/              # HTTP → gRPC прокси
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── order-service/            # Управление заказами
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── payment-service/          # Обработка платежей
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── delivery-service/         # Управление доставкой
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── docker-compose.yml        # Полный стек
└── Makefile                  # Удобные команды
```

## Что можно добавить

### Наблюдаемость (Observability)
- **Prometheus + Grafana** — метрики (`prometheus/client_golang`)
- **Jaeger / OpenTelemetry** — distributed tracing через все сервисы
- **Loki** — централизованные логи

### Надёжность
- **Dead Letter Queue (DLQ)** — Kafka-топик для сообщений, которые не удалось обработать
- **Circuit Breaker** — `sony/gobreaker` для защиты от каскадных падений
- **gRPC Retry** — автоматические повторы при временных ошибках

### Безопасность
- **JWT авторизация** в API Gateway (`golang-jwt/jwt`)
- **mTLS** между gRPC сервисами
- **Vault** для управления секретами

### Масштабирование
- **Kubernetes** манифесты (Deployment + Service + HPA)
- **Redis** кэш для GET-запросов статусов
- **Партиционирование Kafka** по user_id для порядка событий