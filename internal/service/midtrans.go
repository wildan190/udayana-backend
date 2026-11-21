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

// midtransOrderID membangun order_id yang stabil, dipakai konsisten baik saat
// membuat charge (POST /charge) maupun saat memeriksa status (GET /{order_id}/status).
func midtransOrderID(orderID int64) string {
	return fmt.Sprintf("ORDER-%d", orderID)
}

// statusCode mengambil status_code dari response Midtrans (string).
func statusCode(result map[string]any) string {
	code, _ := result["status_code"].(string)
	return code
}

// isApprovedStatus menandakan transaksi Midtrans berhasil dibuat/ditemukan.
// Charge sukses memakai 201, sedangkan endpoint status memakai 200/201.
func isApprovedStatus(code string) bool {
	return code == "200" || code == "201"
}

func (m *MidtransClient) Charge(ctx context.Context, order domain.Order, customer domain.User) (map[string]any, error) {
	baseURL := "https://api.sandbox.midtrans.com/v2/charge"
	if m.isProduction {
		baseURL = "https://api.midtrans.com/v2/charge"
	}

	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     midtransOrderID(order.ID),
			"gross_amount": order.TotalCents / 100,
		},
		"customer_details": map[string]any{
			"first_name": customer.Name,
			"email":      customer.Email,
			"phone":      customer.Phone,
		},
	}

	// item_details wajib berjumlah sama dengan gross_amount. Karena total
	// pesanan sudah termasuk ongkos kirim, ongkir dimasukkan sebagai item
	// tersendiri agar Midtrans tidak menolak transaksi.
	itemDetails := make([]map[string]any, 0, len(order.Items)+1)
	for _, item := range order.Items {
		itemDetails = append(itemDetails, map[string]any{
			"id":       fmt.Sprintf("%d", item.ProductID),
			"price":    item.PriceCents / 100,
			"quantity": item.Quantity,
			"name":     item.ProductName,
		})
	}
	if order.ShippingFeeCents > 0 {
		itemDetails = append(itemDetails, map[string]any{
			"id":       "SHIPPING",
			"price":    order.ShippingFeeCents / 100,
			"quantity": 1,
			"name":     "Ongkos Kirim",
		})
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
	if !isApprovedStatus(statusCode(result)) {
		message, _ := result["status_message"].(string)
		return result, fmt.Errorf("midtrans menolak charge (status %s): %s", statusCode(result), message)
	}
	return result, nil
}

func (m *MidtransClient) CheckStatus(ctx context.Context, orderID int64) (map[string]any, error) {
	baseURL := fmt.Sprintf("https://api.sandbox.midtrans.com/v2/%s/status", midtransOrderID(orderID))
	if m.isProduction {
		baseURL = fmt.Sprintf("https://api.midtrans.com/v2/%s/status", midtransOrderID(orderID))
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
