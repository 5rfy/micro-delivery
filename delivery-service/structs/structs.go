package structs

import "github.com/shopspring/decimal"

type PaymentCompletedEvent struct {
	OrderId string          `json:"order_id"`
	UserId  string          `json:"user_id"`
	Amount  decimal.Decimal `json:"amount"`
	Status  string          `json:"status"`
}

type DeliveryStatusUpdatedEvent struct {
	OrderId        string `json:"order_id"`
	DeliveryId     string `json:"delivery_id"`
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number"`
	EstimatedDate  string `json:"estimated_date"`
}
