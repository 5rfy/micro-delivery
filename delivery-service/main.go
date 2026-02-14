package main

import (
	"log"
	"net"
	"os"

	"github.com/5rfy/micro-delivery/proto/generated/delivery"
	"github.com/IBM/sarama"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"main.go/database"
	"main.go/kafka"
	"main.go/service"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=deliveries sslmode=disable"
	}

	kafkaBrokers := []string{os.Getenv("KAFKA_BROKERS")}
	if kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}

	db := database.InitDb(dsn)

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll

	producer, err := sarama.NewSyncProducer(kafkaBrokers, config)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()

	go kafka.StartConsumer(db, producer, kafkaBrokers)

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50053"
	}

	listen, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	delivery.RegisterDeliveryServiceServer(grpcServer, service.NewServer(db, producer))
	reflection.Register(grpcServer)

	log.Printf("Delivery Service gRPC listening on :%s", port)
	if err := grpcServer.Serve(listen); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
