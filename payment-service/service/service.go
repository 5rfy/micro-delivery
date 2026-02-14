package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/5rfy/micro-delivery/proto/generated/payment"
	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"main.go/structs"
)

const TopicPaymentCompleted = "payment.completed"

type Server struct {
	payment.UnimplementedPaymentServiceServer
	db       *sql.DB
	producer sarama.SyncProducer
}

func NewServer(db *sql.DB, producer sarama.SyncProducer) *Server {
	return &Server{db: db, producer: producer}
}

func (s *Server) ProcessPayment(ctx context.Context, req *payment.ProcessPaymentRequest) (*payment.ProcessPaymentResponse, error) {
	paymentId := uuid.New().String()

	var balance decimal.Decimal
	err := s.db.QueryRowContext(ctx,
		`SELECT balance FROM user_balances WHERE user_id = $1`,
		req.UserId,
	).Scan(&balance)

	if errors.Is(err, sql.ErrNoRows) {
		balance = decimal.NewFromFloat(1000.0)
		s.db.ExecContext(ctx,
			`INSERT INTO user_balances (user_id, balance) VALUES ($1, $2)`,
			req.UserId, balance,
		)
	} else if err != nil {
		return nil, fmt.Errorf("failed to check balance: %w", err)
	}

	var status, msg string
	cost := decimal.NewFromFloat(req.Amount)

	if cost.GreaterThan(balance) {
		status = "INSUFFICIENT_FUNDS"
		msg = fmt.Sprintf("Insufficient funds. Balance: %s, Required: %s", balance, cost)

		_, err := s.db.ExecContext(ctx,
			`INSERT INTO payments (id, order_id, user_id, amount, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			paymentId, req.OrderId, req.UserId, req.Amount, status, time.Now(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert payment: %w", err)
		}
	} else {
		tx, _ := s.db.BeginTx(ctx, nil)
		defer tx.Rollback()

		_, err := tx.ExecContext(ctx,
			`UPDATE user_balances SET balance = balance - $1 WHERE user_id = $2`,
			req.Amount, req.UserId,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to debuct balance: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO payments (id, order_id, user_id, amount, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			paymentId, req.OrderId, req.UserId, req.Amount, status, time.Now(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert payment: %w", err)
		}

		err = tx.Commit()

		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		status = "SUCCESS"
		msg = fmt.Sprintf("Payment of %s processed successfully", cost)
	}

	event := structs.PaymentCompletedEvent{
		OrderId:   req.OrderId,
		PaymentId: paymentId,
		UserId:    req.UserId,
		Amount:    cost,
		Status:    status,
	}
	payload, _ := json.Marshal(event)

	go s.publishEvent(TopicPaymentCompleted, req.OrderId, payload)

	return &payment.ProcessPaymentResponse{
		PaymentId: paymentId,
		Status:    status,
		Message:   msg,
	}, nil
}

func (s *Server) GetPaymentStatus(ctx context.Context, req *payment.GetPaymentStatusRequest) (*payment.GetPaymentStatusResponse, error) {
	var res payment.GetPaymentStatusResponse
	var paidAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, order_id, status, amount, created_at 
		FROM payments 
		WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`,
		req.OrderId,
	).Scan(&res.PaymentId, &res.Status, &res.Amount, &paidAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("payment for order %s not found", req.OrderId)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get payment status: %w", err)
	}

	res.PaidAt = paidAt.Format(time.RFC3339)
	return &res, nil
}

func (s *Server) publishEvent(eventTopic string, orderId string, payload []byte) {
	msg := &sarama.ProducerMessage{
		Topic: eventTopic,
		Key:   sarama.StringEncoder(orderId),
		Value: sarama.ByteEncoder(payload),
	}

	_, _, err := s.producer.SendMessage(msg)
	if err != nil {
		log.Printf("Failed to publish to %s: %v", eventTopic, err)
	}
}
