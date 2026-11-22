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

func (i *IrisClient) CreatePayout(ctx context.Context, beneficiaryName, beneficiaryAccount, beneficiaryBank, amount string, notes string) (string, error) {
	baseURL := "https://app.sandbox.midtrans.com/iris/api/v1/payouts"
	if i.isProduction {
		baseURL = "https://app.midtrans.com/iris/api/v1/payouts"
	}

	payload := map[string]any{
		"payouts": []map[string]any{
			{
				"beneficiary_name":    beneficiaryName,
				"beneficiary_account": beneficiaryAccount,
				"beneficiary_bank":    beneficiaryBank,
				"amount":              amount,
				"notes":               notes,
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(i.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	status, _ := result["status"].(string)
	if status != "success" && status != "queued" && status != "processing" {
		return "", fmt.Errorf("iris payout rejected: %+v", result)
	}

	payouts, _ := result["payouts"].([]any)
	if len(payouts) > 0 {
		if pMap, ok := payouts[0].(map[string]any); ok {
			refNo, _ := pMap["reference_no"].(string)
			if refNo != "" {
				return refNo, nil
			}
		}
	}

	return "IRIS-SUCCESS-REF", nil
}
