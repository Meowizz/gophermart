package worker

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"fmt"

	"github.com/Meowizz/gophermart/internal/repository"
	"github.com/shopspring/decimal"
)

type AccurualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

func StatusAccrualWorker(ctx context.Context, store *repository.Store, accurualURL string) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		log.Println("Accurual worker started")

		for {
			select {
			case <-ctx.Done():
				log.Println("Accurual worker stopped")
			case <-ticker.C:
				processOrders(ctx, store, accurualURL, client)
			}
		}
	}()
}

func processOrders(ctx context.Context, store *repository.Store, accurualURL string, client *http.Client) {
	orders, err := store.GetOrdersForProcessing(ctx, 10)

	if err != nil {
		log.Printf("worker: failed to get orders for processing: %v", err)
		return
	}

	for _, order := range orders {
		if ctx.Err() != nil {
			return
		}
		reqURL := fmt.Sprintf("%s/api/orders/%s", accurualURL, order.Number)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			log.Printf("worker:failed to create request for %s: %v", order.Number, err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("worker: request failed for %s: %v", order.Number, err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfterStr := resp.Header.Get("Retry-After")
			retryAfter, err := strconv.Atoi(retryAfterStr)
			if err != nil || retryAfter <= 0 {
				retryAfter = 60
			}
			log.Printf("worker: rate limited for %s, waiting %d seconds", order.Number, retryAfter)

			select {
			case <-time.After(time.Duration(retryAfter) * time.Second):
				resp.Body.Close()
				return
			case <-ctx.Done():
				resp.Body.Close()
				return
			}
		}

		if resp.StatusCode == http.StatusOK {
			var accurualResp AccurualResponse
			if err := json.NewDecoder(resp.Body).Decode(&accurualResp); err != nil {
				log.Printf("worker: failed to decode response for %s: %v", order.Number, err)
				resp.Body.Close()
				continue
			}
			resp.Body.Close()
			switch accurualResp.Status {
			case "PROCESSED":
				accurualValue := decimal.Zero
				if accurualResp.Accrual != nil {
					accurualValue = decimal.NewFromFloat(*accurualResp.Accrual)
				}
				if err := store.ProcessOrderAccurual(ctx, order.Number, accurualValue); err != nil {
					log.Printf("worker: failed to process accrual for %s: %v", order.Number, err)
				} else {
					log.Printf("worker: successfully processed accrual for %s: %s", order.Number, accurualValue.String())
				}
			case "INVALID":
				if err := store.UpdateOrderStatus(ctx, order.Number, "INVALID"); err != nil {
					log.Printf("worker: failed to mark order %s as INVALID: %v", order.Number, err)
				} else {
					log.Printf("worker: marked order %s as INVALID", order.Number)
				}
			case "REGISTERED", "PROCESSING":
				if err := store.UpdateOrderStatus(ctx, order.Number, "PROCESSING"); err != nil {
					log.Printf("worker: failed to mark order %s as PROCESSING: %v", order.Number, err)
				} else {
					log.Printf("worker: marked order %s as PROCESSING (waiting)", order.Number)
				}
			}
		} else {
			log.Printf("worker: unexpected status code %d for order %s", resp.StatusCode, order.Number)
			resp.Body.Close()
		}
	}
}
