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

type IrisClient struct {
	isProduction bool
	serverKey    string
}

func NewIrisClient() *IrisClient {
	env := os.Getenv("MIDTRANS_ENV")
	isProd := env == "production" || env == "prod"
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	return &IrisClient{
		isProduction: isProd,
		serverKey:    serverKey,
	}
}

func (i *IrisClient) CreatePayout(ctx context.Context, withdrawal domain.Withdrawal) (map[string]any, error) {
	baseURL := "https://app.sandbox.midtrans.com/iris/api/v1/payouts"
	if i.isProduction {
		baseURL = "https://app.midtrans.com/iris/api/v1/payouts"
	}

	payload := map[string]any{
		"payouts": []map[string]any{
			{
				"beneficiary_name":    withdrawal.AccountName,
				"beneficiary_account": withdrawal.AccountNumber,
				"beneficiary_bank":    withdrawal.BankName,
				"amount":             withdrawal.AmountCents / 100,
				"notes":              "Penarikan Dana Udayana Market",
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(i.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Idempotency-Key", fmt.Sprintf("WD-%d-%d", withdrawal.AdminID, time.Now().UnixNano()))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
