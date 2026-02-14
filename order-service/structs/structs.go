package structs

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderCreatedEvent struct {
	OrderId         string          `json:"order_id"`
	UserId          string          `json:"user_id"`
	Items           []*OrderItem    `json:"items"`
	TotalAmount     decimal.Decimal `json:"total_amount"`
	DeliveryAddress string          `json:"delivery_address"`
	CreatedAt       time.Time       `json:"created_at"`
}

type OrderItem struct {
	ProductId string          `json:"product_id"`
	Quantity  int32           `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
}

type PaymentCompletedEvent struct {
	OrderId   string `json:"order_id"`
	Status    string `json:"status"`
	PaymentId string `json:"payment_id"`
}

type DeliveryStatusEvent struct {
	OrderId        string `json:"order_id"`
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number"`
	EstimatedDate  string `json:"estimated_delivery"`
}
