package converter

import (
	"time"

	"github.com/Meowizz/gophermart/internal/models"
	"github.com/shopspring/decimal"
)

func ConvertOrderToResponse(order models.Order) models.OrderResponse {
	resp := models.OrderResponse{
		Number:     order.Number,
		Status:     order.Status,
		UploadedAt: order.UploadedAt.Format(time.RFC3339),
	}
	if order.Accrual.GreaterThan(decimal.Zero) {
		accrual := order.Accrual
		resp.Accrual = &accrual
	}
	return resp
}

func ConvertOrdersToResponse(orders []models.Order) []models.OrderResponse {
	responses := make([]models.OrderResponse, len(orders))
	for i, order := range orders {
		responses[i] = ConvertOrderToResponse(order)
	}
	return responses
}
