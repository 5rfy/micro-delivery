package structs

import "github.com/shopspring/decimal"

type Item struct {
	ProductId string          `json:"productId"`
	Quantity  int32           `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
}

type Order struct {
	Items           []Item `json:"items"`
	UserID          string `json:"user_id"`
	DeliveryAddress string `json:"delivery_address"`
}
