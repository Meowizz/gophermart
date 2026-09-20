package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type User struct {
	ID       int
	Login    string
	Password string
}

type Order struct {
	Number     string
	UserID     int
	Status     string
	Accrual    decimal.Decimal
	UploadedAt time.Time
}

type OrderResponse struct {
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt string           `json:"uploaded_at"`
}

type Balance struct {
	UserID    int
	Current   decimal.Decimal
	Withdrawn decimal.Decimal
}

type Withdrawal struct {
	ID          int
	UserID      int
	OrderNumber string
	Sum         decimal.Decimal
	ProcessedAt time.Time
}
