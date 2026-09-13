package converter

import (
	"time"

	"github.com/Meowizz/gophermart/internal/models"
)

func ConvertOrderToResponse(order models.Order) models.OrderResponse {
	return models.OrderResponse{
		Number:     order.Number,
		Status:     order.Status,
		Accrual:    order.Accrual,
		UploadedAt: order.UploadedAt.Format(time.RFC3339),
	}
}

func ConvertOrdersToResponse(orders []models.Order) []models.OrderResponse {
	responses := make([]models.OrderResponse, len(orders))
	for i, order := range orders {
		responses[i] = ConvertOrderToResponse(order)
	}
	return responses
}
