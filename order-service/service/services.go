package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/5rfy/micro-delivery/proto/generated/order"
	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"main.go/kafka"
	"main.go/structs"
)

type Server struct {
	order.UnimplementedOrderServiceServer
	db       *sql.DB
	producer sarama.SyncProducer
}

func NewServer(db *sql.DB, producer sarama.SyncProducer) *Server {
	return &Server{db: db, producer: producer}
}

func (s *Server) CreateOrder(ctx context.Context, req *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	orderId := uuid.New().String()
	totalAmount := decimal.Zero

	for _, item := range req.Items {
		totalAmount = totalAmount.Add(decimal.NewFromFloat(item.Price).Mul(decimal.NewFromInt32(item.Quantity)))
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func(tx *sql.Tx) {
		err := tx.Rollback()
		if err != nil {
			log.Printf("failed to rollback transaction: %w", err)
		}
	}(tx)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO orders (id, user_id, status, total_amount, delivery_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		orderId, req.UserId, "PENDING", totalAmount, req.DeliveryAddress, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to insert order: %w", err)
	}

	items := make([]*structs.OrderItem, len(req.Items))

	for _, item := range req.Items {
		items = append(items, &structs.OrderItem{
			ProductId: item.ProductId,
			Quantity:  item.Quantity,
			Price:     decimal.NewFromFloat(item.Price),
		})
	}

	event := &structs.OrderCreatedEvent{
		OrderId:         orderId,
		UserId:          req.UserId,
		Items:           items,
		TotalAmount:     totalAmount,
		DeliveryAddress: req.DeliveryAddress,
		CreatedAt:       time.Now(),
	}

	eventJson, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (id, topic, payload, created_at) VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), kafka.TopicOrderCreated, string(eventJson), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	go s.publishEvent(kafka.TopicOrderCreated, orderId, eventJson)

	log.Printf("Order created: %s for user: %s, amount: %s", orderId, req.UserId, totalAmount)

	return &order.CreateOrderResponse{
		OrderId: orderId,
		Status:  "PENDING",
		Message: "Order created successfully, processing payment",
	}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	var response order.GetOrderResponse
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, status, total_amount, created_at FROM orders WHERE id = $1`,
		req.OrderId,
	).Scan(&response.OrderId, &response.UserId, &response.TotalAmount, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("response %s not found", req.OrderId)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	response.CreatedAt = createdAt.Format(time.RFC3339)
	return &response, nil
}

func (s *Server) GetDeliveryStatus(ctx context.Context, req *order.GetDeliveryStatusRequest) (*order.GetDeliveryStatusResponse, error) {
	var response order.GetDeliveryStatusResponse

	err := s.db.QueryRowContext(ctx,
		`SELECT o.id, d.status, d.tracking_number, d.estimated_delivery
		 FROM orders o
		 LEFT JOIN delivery_statuses d ON o.id = d.order_id
		 WHERE o.id = $1`,
		req.OrderId,
	).Scan(&response.OrderId, &response.DeliveryStatus, &response.TrackingNumber, &response.EstimatedDelivery)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("response %s not found", req.OrderId)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}

	return &response, nil
}

func (s *Server) publishEvent(topic string, key string, payload []byte) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(payload),
	}

	if _, _, err := s.producer.SendMessage(msg); err != nil {
		log.Printf("Failed to publish event to topic %s: %v", topic, err)
	} else {
		log.Printf("Event published to topic: %s", topic)
	}
}
