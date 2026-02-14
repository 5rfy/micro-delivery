package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/5rfy/micro-delivery/proto/generated/delivery"
	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"main.go/structs"
)

const TopicDeliveryUpdated = "delivery.status.updated"

type Server struct {
	delivery.UnimplementedDeliveryServiceServer
	db       *sql.DB
	producer sarama.SyncProducer
}

func NewServer(db *sql.DB, producer sarama.SyncProducer) *Server {
	return &Server{
		db:       db,
		producer: producer,
	}
}

func (s *Server) CreateDelivery(ctx context.Context, req *delivery.CreateDeliveryRequest) (*delivery.CreateDeliveryResponse, error) {
	deliveryId := uuid.New().String()
	trackingNumber := generateTrackingNumber()
	estimatedDelivery := time.Now().Add(3 * 24 * time.Hour).Format("2006-01-02")

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deliveries (id, order_id, user_id, delivery_address, status, tracking_number, estimated_delivery, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		deliveryId, req.OrderId, req.UserId, req.DeliveryAddress,
		"PENDING", trackingNumber, estimatedDelivery, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("error inserting delivery %v", err)
	}

	go s.simulateDeliveryProgress(req.OrderId, deliveryId, trackingNumber, estimatedDelivery)

	log.Printf("Delivery %s created for order %s, tracking: %s", deliveryId, req.OrderId, trackingNumber)

	return &delivery.CreateDeliveryResponse{
		DeliveryId:        deliveryId,
		TrackingNumber:    trackingNumber,
		Status:            "PENDING",
		EstimatedDelivery: estimatedDelivery,
	}, nil
}

func (s *Server) GetDeliveryStatus(ctx context.Context, req *delivery.GetDeliveryStatusRequest) (*delivery.GetDeliveryStatusResponse, error) {
	var resp delivery.GetDeliveryStatusResponse

	err := s.db.QueryRowContext(ctx,
		`SELECT id, order_id, status, tracking_number, estimated_delivery, current_location
		 FROM deliveries WHERE order_id = $1`,
		req.OrderId,
	).Scan(&resp.DeliveryId, &resp.OrderId, &resp.Status, &resp.TrackingNumber,
		&resp.EstimatedDelivery, &resp.CurrentLocation)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delivery for order %s not found", req.OrderId)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get delivery status: %w", err)
	}

	return &resp, nil
}

func (s *Server) simulateDeliveryProgress(orderId string, deliveryId string, trackingNumber string, estimatedDelivery string) {
	statuses := []struct {
		status   string
		location string
		delay    time.Duration
	}{
		{"IN_TRANSIT", "Warehouse A - Processed", 5 * time.Second},
		{"IN_TRANSIT", "Sorting Center", 10 * time.Second},
		{"OUT_FOR_DELIVERY", "Local Courier Hub", 15 * time.Second},
		{"DELIVERED", "Delivered to address", 20 * time.Second},
	}

	for _, val := range statuses {
		time.Sleep(val.delay)

		s.db.Exec(
			`UPDATE deliveries SET val = $1, current_location = $2, updated_at = $3 WHERE order_id = $4`,
			val.status, val.location, time.Now(), orderId,
		)

		event := structs.DeliveryStatusUpdatedEvent{
			OrderId:        orderId,
			DeliveryId:     deliveryId,
			Status:         val.status,
			TrackingNumber: trackingNumber,
			EstimatedDate:  estimatedDelivery,
		}
		payload, _ := json.Marshal(event)
		s.publishEvent(TopicDeliveryUpdated, orderId, payload)

		log.Printf("Delivery update for order %s: %s at %s", orderId, val.status, val.location)
	}
}

func (s *Server) publishEvent(topic, key string, payload []byte) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(payload),
	}
	_, _, err := s.producer.SendMessage(msg)
	if err != nil {
		log.Printf("Failed to publish to %s: %v", topic, err)
	}
}

func generateTrackingNumber() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 12)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}
