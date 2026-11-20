package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"udayana-backend/internal/domain"
)

type MidtransClient struct {
	isProduction bool
	serverKey    string
	clientKey    string
}

func NewMidtransClient() *MidtransClient {
	env := os.Getenv("MIDTRANS_ENV")
	isProd := env == "production" || env == "prod"
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	clientKey := os.Getenv("MIDTRANS_CLIENT_KEY")

	return &MidtransClient{
		isProduction: isProd,
		serverKey:    serverKey,
		clientKey:    clientKey,
	}
}

func (m *MidtransClient) Charge(ctx context.Context, order domain.Order, customer domain.User) (map[string]any, error) {
	baseURL := "https://api.sandbox.midtrans.com/v2/charge"
	if m.isProduction {
		baseURL = "https://api.midtrans.com/v2/charge"
	}

	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     fmt.Sprintf("ORDER-%d-%d", order.ID, time.Now().Unix()),
			"gross_amount": order.TotalCents / 100,
		},
		"customer_details": map[string]any{
			"first_name": customer.Name,
			"email":      customer.Email,
			"phone":      customer.Phone,
		},
	}

	itemDetails := make([]map[string]any, len(order.Items))
	for i, item := range order.Items {
		itemDetails[i] = map[string]any{
			"id":       fmt.Sprintf("%d", item.ProductID),
			"price":    item.PriceCents / 100,
			"quantity": item.Quantity,
			"name":     item.ProductName,
		}
	}
	payload["item_details"] = itemDetails

	if order.PaymentMethod == "MIDTRANS_QRIS" {
		payload["payment_type"] = "qris"
		payload["qris"] = map[string]any{
			"acquirer": "gopay",
		}
	} else {
		payload["payment_type"] = "bank_transfer"
		payload["bank_transfer"] = map[string]any{
			"bank": "bca",
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *MidtransClient) CheckStatus(ctx context.Context, orderID int64) (map[string]any, error) {
	baseURL := fmt.Sprintf("https://api.sandbox.midtrans.com/v2/ORDER-%d/status", orderID)
	if m.isProduction {
		baseURL = fmt.Sprintf("https://api.midtrans.com/v2/ORDER-%d/status", orderID)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
