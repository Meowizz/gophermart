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
	Number     int
	UserID     int
	Status     string
	Accrual    float64
	UploadedAt time.Time
}
