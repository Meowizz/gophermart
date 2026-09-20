package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Meowizz/gophermart/internal/middleware"
	"github.com/Meowizz/gophermart/internal/repository"
	"github.com/shopspring/decimal"
)

type BalanceResponse struct {
	Current   decimal.Decimal `json:"current"`
	Withdrawn decimal.Decimal `json:"withdrawn"`
}

type WithdrawalRequest struct {
	Order string          `json:"order"`
	Sum   decimal.Decimal `json:"sum"`
}

type WithdrawalResponse struct {
	Order       string          `json:"order"`
	Sum         decimal.Decimal `json:"sum"`
	ProcessedAt string          `json:"processed_at"`
}

func (h *Handler) GetBalance(rw http.ResponseWriter, rq *http.Request) {
	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	balance, err := h.store.GetBalance(rq.Context(), user.ID)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	resp := BalanceResponse{
		Current:   balance.Current,
		Withdrawn: balance.Withdrawn,
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

func (h *Handler) WithdrawBalance(rw http.ResponseWriter, rq *http.Request) {
	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	var req WithdrawalRequest
	if err := json.NewDecoder(rq.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid Request", http.StatusBadRequest)
		return
	}

	if req.Order == "" {
		http.Error(rw, "Invalid order number", http.StatusUnprocessableEntity)
		return
	}
	if req.Sum.LessThanOrEqual(decimal.Zero) {
		http.Error(rw, "Invalid sum", http.StatusUnprocessableEntity)
		return
	}
	order, err := h.store.GetOrderByNumber(rq.Context(), req.Order)
	if err != nil || order.UserID != user.ID {
		http.Error(rw, "Invalid order number", http.StatusUnprocessableEntity)
		return
	}

	err = h.store.WithdrawBalance(rq.Context(), user.ID, req.Sum, req.Order)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientFunds) {
			http.Error(rw, "Insufficient funds", http.StatusPaymentRequired)
		} else {
			log.Printf("Withdraw error : %v", err)
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func (h *Handler) GetWithdrawals(rw http.ResponseWriter, rq *http.Request) {
	userLogin, ok := middleware.GetUserLogin(rq)
	if !ok {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.store.GetUserByLogin(rq.Context(), userLogin)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	withdrawals, err := h.store.GetWithdrawals(rq.Context(), user.ID)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		rw.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]WithdrawalResponse, len(withdrawals))
	for i, w := range withdrawals {
		resp[i] = WithdrawalResponse{
			Order:       w.OrderNumber,
			Sum:         w.Sum,
			ProcessedAt: w.ProcessedAt.Format(time.RFC3339),
		}
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}
