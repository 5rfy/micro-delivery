package structs

import "github.com/shopspring/decimal"

type OrderCreatedEvent struct {
	OrderId         string          `json:"order_id"`
	UserId          string          `json:"user_id"`
	TotalAmount     decimal.Decimal `json:"total_amount"`
	DeliveryAddress string          `json:"delivery_address"`
}

type PaymentCompletedEvent struct {
	OrderId   string          `json:"order_id"`
	PaymentId string          `json:"payment_id"`
	UserId    string          `json:"user_id"`
	Amount    decimal.Decimal `json:"amount"`
	Status    string          `json:"status"`
}
