proto:
	@echo "Generating gRPC code from proto files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/order.proto proto/payment.proto proto/delivery.proto
	@echo "Done!"

