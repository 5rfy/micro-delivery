package kafka

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/5rfy/micro-delivery/proto/generated/payment"
	"github.com/IBM/sarama"
	"main.go/service"
	"main.go/structs"
)

const TopicOrderCreated = "order.created"

type ConsumerHandler struct {
	db       *sql.DB
	producer sarama.SyncProducer
}

func (h *ConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}
func (h *ConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Payment service received order.created event")

		var event structs.OrderCreatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("Failed to unmarshal: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		s := service.NewServer(h.db, h.producer)
		_, err := s.ProcessPayment(context.Background(), &payment.ProcessPaymentRequest{
			OrderId:  event.OrderId,
			UserId:   event.UserId,
			Amount:   event.TotalAmount.InexactFloat64(),
			Currency: "USD"},
		)
		if err != nil {
			return fmt.Errorf("failed to process: %v", err)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

func StartConsumer(db *sql.DB, producer sarama.SyncProducer, brokers []string) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}

	group, err := sarama.NewConsumerGroup(brokers, "payment-service-group", config)
	if err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}

	handler := &ConsumerHandler{db: db, producer: producer}

	for {
		if err := group.Consume(context.Background(), []string{TopicOrderCreated}, handler); err != nil {
			log.Printf("Consumer error: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}
