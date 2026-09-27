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

type AccuralResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accural *float64 `json:"accural,omitempty"`
}

func StatusAccuralWorker(ctx context.Context, store *repository.Store, accuralURL string) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		log.Println("Accural worker started")

		for {
			select {
			case <-ctx.Done():
				log.Println("Accural worker stopped")
			case <-ticker.C:
				processOrders(ctx, store, accuralURL, client)
			}
		}
	}()
}

func processOrders(ctx context.Context, store *repository.Store, accuralURL string, client *http.Client) {
	orders, err := store.GetOrdersForProcessing(ctx, 10)

	if err != nil {
		log.Printf("worker: failed to get orders for processing: %v", err)
		return
	}

	for _, order := range orders {
		if ctx.Err() != nil {
			return
		}
		reqURL := fmt.Sprintf("%s/api/orders/%s", accuralURL, order.Number)
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
			var accuralResp AccuralResponse
			if err := json.NewDecoder(resp.Body).Decode(&accuralResp); err != nil {
				log.Printf("worker: failed to decode response for %s: %v", order.Number, err)
				resp.Body.Close()
				continue
			}
			resp.Body.Close()
			switch accuralResp.Status {
			case "PROCESSED":
				accuralValue := decimal.Zero
				if accuralResp.Accural != nil {
					accuralValue = decimal.NewFromFloat(*accuralResp.Accural)
				}
				if err := store.ProcessOrderAccural(ctx, order.Number, accuralValue); err != nil {
					log.Printf("worker: failed to process accrual for %s: %v", order.Number, err)
				} else {
					log.Printf("worker: successfully processed accrual for %s: %s", order.Number, accuralValue.String())
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
