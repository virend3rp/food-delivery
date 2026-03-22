.PHONY: up down logs build tidy test-order health

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

build:
	go build ./...

tidy:
	go mod tidy

# Fire a test order at the API gateway
test-order:
	curl -s -X POST http://localhost:8080/orders \
		-H "Content-Type: application/json" \
		-d '{ \
			"customer_id": "cust-001", \
			"restaurant_id": "rest-burger-palace", \
			"items": [ \
				{"name": "Double Burger", "quantity": 2, "price": 9.99}, \
				{"name": "Fries",         "quantity": 1, "price": 3.49}, \
				{"name": "Coke",          "quantity": 2, "price": 1.99} \
			] \
		}' | jq

health:
	curl -s http://localhost:8080/health | jq
