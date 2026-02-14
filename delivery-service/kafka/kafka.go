package kafka

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/5rfy/micro-delivery/proto/generated/delivery"
	"github.com/IBM/sarama"
	"main.go/service"
	"main.go/structs"
)

const TopicPaymentCompleted = "payment.completed"

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
		var event structs.PaymentCompletedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("Failed to unmarshal payment event: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		if event.Status == "SUCCESS" {
			log.Printf("Payment success for order %s — creating delivery", event.OrderId)
			s := service.NewServer(h.db, h.producer)
			_, err := s.CreateDelivery(context.Background(), &delivery.CreateDeliveryRequest{
				OrderId:         event.OrderId,
				UserId:          event.UserId,
				DeliveryAddress: "Default Address",
			})
			if err != nil {
				log.Printf("Failed to create delivery: %v", err)
			}
		} else {
			log.Printf("Payment failed for order %s (%s) — no delivery created", event.OrderId, event.Status)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

func StartConsumer(db *sql.DB, producer sarama.SyncProducer, brokers []string) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}

	group, err := sarama.NewConsumerGroup(brokers, "delivery-service-group", config)
	if err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}

	handler := &ConsumerHandler{db: db, producer: producer}
	for {
		if err := group.Consume(context.Background(), []string{TopicPaymentCompleted}, handler); err != nil {
			log.Fatalf("Error on consume from group: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}
