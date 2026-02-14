.PHONY: proto build up down logs test clean

# ─── Proto generation ──────────────────────────────────────────
proto:
	@echo "Generating gRPC code from proto files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/order.proto proto/payment.proto proto/delivery.proto
	@echo "Done!"

# ─── Docker ────────────────────────────────────────────────────
up:
	docker compose up --build -d
	@echo ""
	@echo "Services started:"
	@echo "  API Gateway:  http://localhost:8080"
	@echo "  Kafka UI:     http://localhost:8090"
	@echo "  Order gRPC:   localhost:50051"
	@echo "  Payment gRPC: localhost:50052"
	@echo "  Delivery gRPC:localhost:50053"

down:
	docker compose down

down-volumes:
	docker compose down -v

logs:
	docker compose logs -f

logs-order:
	docker compose logs -f order-service

logs-payment:
	docker compose logs -f payment-service

logs-delivery:
	docker compose logs -f delivery-service

logs-gateway:
	docker compose logs -f api-gateway

# ─── Testing ──────────────────────────────────────────────────
# Create a test order
test-create-order:
	@echo "Creating test order..."
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{ \
			"user_id": "user-123", \
			"delivery_address": "ul. Lenina 1, Moscow", \
			"items": [ \
				{"product_id": "prod-1", "quantity": 2, "price": 149.99}, \
				{"product_id": "prod-2", "quantity": 1, "price": 299.00} \
			] \
		}' | jq .

# ─── DB access ────────────────────────────────────────────────
db-orders:
	docker compose exec orders-db psql -U postgres -d orders

db-payments:
	docker compose exec payments-db psql -U postgres -d payments

db-deliveries:
	docker compose exec deliveries-db psql -U postgres -d deliveries