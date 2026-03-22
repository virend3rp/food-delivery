# Food Delivery — Event-Driven Microservices

A production-style food delivery backend built with **Go** and **RabbitMQ**, demonstrating event-driven architecture, asynchronous messaging, and independent service scalability.

## Architecture

```
POST /orders
     │
     ▼
┌─────────────┐   order.created   ┌──────────────────────┐
│ api-gateway │──────────────────▶│  restaurant-service  │
│  (HTTP in)  │                   │  Accepts the order   │
└─────────────┘                   └──────────┬───────────┘
                                             │ order.accepted
                                             ▼
                                  ┌──────────────────────┐
                                  │  delivery-service    │
                                  │  Assigns driver      │
                                  └──────────┬───────────┘
                                             │ driver.assigned
                                             │ order.picked_up
                                             │ order.delivered
                                             ▼
                                  ┌──────────────────────┐
                                  │ notification-service │◀── all events
                                  │  Logs every update   │
                                  └──────────────────────┘
```

No service talks directly to another — all communication is via **RabbitMQ events**.

## Services

| Service | Port | Responsibility | DB |
|---|---|---|---|
| `api-gateway` | 8080 | Receives HTTP orders, publishes `order.created` | `orders_db` |
| `restaurant-service` | — | Consumes `order.created`, publishes `order.accepted` | `restaurant_db` |
| `delivery-service` | — | Consumes `order.accepted`, tracks full delivery lifecycle | `delivery_db` |
| `notification-service` | — | Consumes **all** events, persists audit log | `notification_db` |

## Event Flow

| Event | Routing Key | Published By | Consumed By |
|---|---|---|---|
| `OrderCreatedEvent` | `order.created` | api-gateway | restaurant-service, notification-service |
| `OrderAcceptedEvent` | `order.accepted` | restaurant-service | delivery-service, notification-service |
| `OrderRejectedEvent` | `order.rejected` | restaurant-service | notification-service |
| `DriverAssignedEvent` | `driver.assigned` | delivery-service | notification-service |
| `OrderPickedUpEvent` | `order.picked_up` | delivery-service | notification-service |
| `OrderDeliveredEvent` | `order.delivered` | delivery-service | notification-service |

## RabbitMQ Topology

```
Exchange: food_delivery (topic)
   │
   ├── restaurant.queue  ← binds order.created
   ├── delivery.queue    ← binds order.accepted
   └── notification.queue ← binds order.*, driver.*

Dead-letter exchange: food_delivery.dlx (fanout)
   └── food_delivery.dlq  ← failed messages land here
```

**Key patterns used:**
- **Topic exchange** — flexible routing by dot-separated keys
- **Dead-letter queue** — failed messages (nack) auto-route to DLQ for inspection
- **Durable queues** — survive RabbitMQ restarts
- **Manual acknowledgment** — messages acked only after successful processing
- **QoS prefetch = 1** — fair dispatch across multiple worker instances
- **Channel-per-producer/consumer** — safe concurrent usage

## Getting Started

### Prerequisites
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [Go 1.22+](https://golang.org/dl/) (for local development)

### Run the full stack

```bash
# Start RabbitMQ + PostgreSQL + all services
make up

# Watch all logs in real time
make logs

# Fire a test order
make test-order

# Tear everything down
make down
```

### Useful URLs

| URL | Description |
|---|---|
| `http://localhost:8080` | API Gateway |
| `http://localhost:15672` | RabbitMQ Management UI (`guest` / `guest`) |
| `localhost:5432` | PostgreSQL (`postgres` / `postgres`) |

## API Reference

### `POST /orders`

Place a new food order.

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id":   "cust-001",
    "restaurant_id": "rest-burger-palace",
    "items": [
      { "name": "Double Burger", "quantity": 2, "price": 9.99 },
      { "name": "Fries",         "quantity": 1, "price": 3.49 }
    ]
  }'
```

**Response `202 Accepted`:**
```json
{
  "order_id": "a3f1c2d4-...",
  "status":   "pending",
  "message":  "your order is being processed"
}
```

### `GET /orders/:id`

Retrieve an order by ID.

```bash
curl http://localhost:8080/orders/a3f1c2d4-...
```

**Response `200 OK`:**
```json
{
  "id":            "a3f1c2d4-...",
  "customer_id":   "cust-001",
  "restaurant_id": "rest-burger-palace",
  "total_price":   23.47,
  "status":        "pending"
}
```

### `GET /health`

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Database Schema

Each service owns its own database — no cross-service DB access.

### `orders_db` (api-gateway)
```sql
orders       (id, customer_id, restaurant_id, total_price, status, created_at)
order_items  (id, order_id, name, quantity, price)
```

### `restaurant_db` (restaurant-service)
```sql
restaurant_orders (order_id, restaurant_id, status, estimated_time, processed_at)
```

### `delivery_db` (delivery-service)
```sql
deliveries (order_id, driver_id, driver_name, status, assigned_at, picked_up_at, delivered_at)
```

### `notification_db` (notification-service)
```sql
notification_logs (id, order_id, event_type, message, created_at)
```

## Running Tests

```bash
go test ./...
```

Tests are fully unit-tested with interface-based mocks — no RabbitMQ or PostgreSQL required.

```
ok  github.com/virend3rp/food-delivery/cmd/api-gateway
ok  github.com/virend3rp/food-delivery/cmd/delivery-service
ok  github.com/virend3rp/food-delivery/cmd/notification-service
ok  github.com/virend3rp/food-delivery/cmd/restaurant-service
ok  github.com/virend3rp/food-delivery/internal/events
```

## Project Structure

```
food-delivery/
├── cmd/
│   ├── api-gateway/
│   │   ├── main.go          # HTTP server setup
│   │   ├── service.go       # HTTP handlers (testable)
│   │   ├── store.go         # PostgreSQL order persistence
│   │   └── service_test.go
│   ├── restaurant-service/
│   │   ├── main.go
│   │   ├── service.go       # Order acceptance handler
│   │   ├── store.go
│   │   └── service_test.go
│   ├── delivery-service/
│   │   ├── main.go
│   │   ├── service.go       # Full delivery lifecycle handler
│   │   ├── store.go
│   │   └── service_test.go
│   └── notification-service/
│       ├── main.go
│       ├── service.go       # Event router + logger
│       ├── store.go
│       └── service_test.go
├── internal/
│   ├── db/db.go             # PostgreSQL connection pool
│   ├── events/
│   │   ├── events.go        # Shared event types
│   │   └── events_test.go
│   └── rabbitmq/
│       ├── connection.go    # AMQP connection + topology
│       ├── publisher.go     # Publish helper
│       └── consumer.go      # Consume helper (DLQ on error)
├── deployments/             # Dockerfiles per service
├── docker/init-db.sh        # Creates 4 PostgreSQL databases
├── docker-compose.yml
└── Makefile
```

## Tech Stack

| Technology | Role |
|---|---|
| Go 1.22 | All services |
| RabbitMQ 3.13 | Message broker |
| PostgreSQL 16 | Per-service persistence |
| Gin | HTTP framework (api-gateway) |
| amqp091-go | RabbitMQ client |
| pgx/v5 | PostgreSQL driver |
| Docker Compose | Local orchestration |
