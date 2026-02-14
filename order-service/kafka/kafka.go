package kafka

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/IBM/sarama"
)

const (
	TopicOrderCreated      = "order.created"
	TopicPaymentCompleted  = "payment.completed"
	TopicDeliveryCompleted = "delivery.status.updated"
)

type ConsumerGroupHandler struct {
	db *sql.DB
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Received event from topic %s: %s", msg.Topic, string(msg.Value))

		switch msg.Topic {
		case TopicPaymentCompleted:
			h.handlePaymentCompleted(msg.Value)
		case TopicDeliveryCompleted:
			h.handleDeliveryUpdated(msg.Value)
		}

		session.MarkMessage(msg, "")
	}
	return nil
}

type PaymentCompletedEvent struct {
	OrderId   string `json:"order_id"`
	Status    string `json:"status"`
	PaymentId string `json:"payment_id"`
}

func (h *ConsumerGroupHandler) handlePaymentCompleted(data []byte) {
	var event PaymentCompletedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		return
	}

	newStatus := "PAYMENT_FAILED"
	if event.Status == "SUCCESS" {
		newStatus = "PAID"
	} else if event.Status == "INSUFFICIENT_FUNDS" {
		newStatus = "INSUFFICIENT_FUNDS"
	}

	_, err := h.db.Exec(
		`UPDATE orders SET status = $1 WHERE id = $2`,
		newStatus, event.OrderId)
	if err != nil {
		log.Printf("Failed to update order: %v", err)
		return
	}
	log.Printf("Order %s completed successfully", event.OrderId)
}

type DeliveryStatusEvent struct {
	OrderId        string `json:"order_id"`
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number"`
	EstimatedDate  string `json:"estimated_delivery"`
}

func (h *ConsumerGroupHandler) handleDeliveryUpdated(data []byte) {
	var event DeliveryStatusEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		return
	}

	_, err := h.db.Exec(`INSERT INTO delivery_statuses (order_id, status, tracking_number, estimated_delivery)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (order_id) DO UPDATE SET status = $2, tracking_number = $3`,
		event.OrderId, event.Status, event.TrackingNumber, event.EstimatedDate)
	if err != nil {
		log.Printf("Failed to update delivery: %v", err)
		return
	}
	log.Printf("Delivery status for order %s: %s", event.OrderId, event.Status)
}

func StartConsumer(db *sql.DB, brokers []string) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}

	group, err := sarama.NewConsumerGroup(brokers, "order-service-group", config)
	if err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}
	defer group.Close()

	handler := &ConsumerGroupHandler{db: db}
	topics := []string{TopicPaymentCompleted, TopicDeliveryCompleted}

	for {
		if err := group.Consume(context.Background(), topics, handler); err != nil {
			log.Printf("Consumer error: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}
