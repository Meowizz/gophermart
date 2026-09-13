package models

import (
	"time"
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
	Accrual    float64
	UploadedAt time.Time
}

type OrderResponse struct {
	Number     string  `json:"number"`
	Status     string  `json:"status"`
	Accrual    float64 `json:"accrual,omitempty"`
	UploadedAt string  `json:"uploaded_at"`
}
