package redsys

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"evsys-back/internal/lib/sl"
)

const (
	// Transaction types
	TransactionTypeCapture = "2"
	TransactionTypeCancel  = "9"

	// Response codes
	ResponseCodeOK = "0000"
)

// Config holds the Redsys merchant configuration
type Config struct {
	MerchantCode string
	Terminal     string
	SecretKey    string
	RestApiUrl   string
	Currency     string
}

// Client is the Redsys REST API client
type Client struct {
	httpClient *http.Client
	config     Config
	log        *slog.Logger
}

// NewClient creates a new Redsys client
func NewClient(config Config, log *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: config,
		log:    log.With(sl.Module("redsys.client")),
	}
}

// MerchantParameters represents the Ds_MerchantParameters for requests
type MerchantParameters struct {
	MerchantCode      string `json:"Ds_Merchant_MerchantCode"`
	Terminal          string `json:"Ds_Merchant_Terminal"`
	TransactionType   string `json:"Ds_Merchant_TransactionType"`
	Amount            string `json:"Ds_Merchant_Amount"`
	Currency          string `json:"Ds_Merchant_Currency"`
	Order             string `json:"Ds_Merchant_Order"`
	AuthorisationCode string `json:"Ds_Merchant_AuthorisationCode,omitempty"`
}

// Request represents the Redsys REST API request body
type Request struct {
	MerchantParameters string `json:"Ds_MerchantParameters"`
	SignatureVersion   string `json:"Ds_SignatureVersion"`
	Signature          string `json:"Ds_Signature"`
}

// Response represents the Redsys REST API response
type Response struct {
	MerchantParameters string `json:"Ds_MerchantParameters"`
	Signature          string `json:"Ds_Signature"`
}

// DecodedResponse represents the decoded merchant parameters from response
type DecodedResponse struct {
	ResponseCode      string `json:"Ds_Response"`
	AuthorisationCode string `json:"Ds_AuthorisationCode"`
	Order             string `json:"Ds_Order"`
	Amount            string `json:"Ds_Amount"`
	Currency          string `json:"Ds_Currency"`
	ErrorCode         string `json:"Ds_ErrorCode,omitempty"`
}

// CaptureRequest represents a capture request
type CaptureRequest struct {
	OrderNumber       string
	Amount            int
	AuthorizationCode string
}

// CaptureResponse represents a capture response
type CaptureResponse struct {
	Success           bool
	ResponseCode      string
	AuthorizationCode string
	ErrorCode         string
	ErrorMessage      string
}

// Capture performs a capture operation on a preauthorized amount
func (c *Client) Capture(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	return c.performTransaction(ctx, req, TransactionTypeCapture)
}

// Cancel performs a cancellation of a preauthorization
func (c *Client) Cancel(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	return c.performTransaction(ctx, req, TransactionTypeCancel)
}

// performTransaction executes a transaction with the given type
func (c *Client) performTransaction(ctx context.Context, req CaptureRequest, txType string) (*CaptureResponse, error) {
	log := c.log.With(
		slog.String("order", req.OrderNumber),
		slog.Int("amount", req.Amount),
		slog.String("tx_type", txType),
	)

	// Build merchant parameters
	params := MerchantParameters{
		MerchantCode:      c.config.MerchantCode,
		Terminal:          c.config.Terminal,
		TransactionType:   txType,
		Amount:            fmt.Sprintf("%d", req.Amount),
		Currency:          c.config.Currency,
		Order:             req.OrderNumber,
		AuthorisationCode: req.AuthorizationCode,
	}

	// Encode parameters to JSON and then base64
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}
	merchantParams := base64.StdEncoding.EncodeToString(paramsJSON)

	// Generate signature
	signature, err := GenerateSignature(merchantParams, c.config.SecretKey, req.OrderNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signature: %w", err)
	}

	// Build request
	apiReq := Request{
		MerchantParameters: merchantParams,
		SignatureVersion:   "HMAC_SHA256_V1",
		Signature:          signature,
	}

	reqBody, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Debug("sending request to Redsys")

	// Make HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.RestApiUrl, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.With(
			slog.Int("status_code", resp.StatusCode),
			slog.String("body", string(respBody)),
		).Error("Redsys API returned non-OK status")
		return &CaptureResponse{
			Success:      false,
			ErrorCode:    fmt.Sprintf("%d", resp.StatusCode),
			ErrorMessage: string(respBody),
		}, nil
	}

	// Parse response
	var apiResp Response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Decode merchant parameters from response
	decodedParams, err := base64.StdEncoding.DecodeString(apiResp.MerchantParameters)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response parameters: %w", err)
	}

	var decoded DecodedResponse
	if err := json.Unmarshal(decodedParams, &decoded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decoded parameters: %w", err)
	}

	// Check response code
	success := decoded.ResponseCode == ResponseCodeOK || (len(decoded.ResponseCode) == 4 && decoded.ResponseCode[0] == '0')

	log.With(
		slog.String("response_code", decoded.ResponseCode),
		slog.Bool("success", success),
	).Info("Redsys transaction completed")

	return &CaptureResponse{
		Success:           success,
		ResponseCode:      decoded.ResponseCode,
		AuthorizationCode: decoded.AuthorisationCode,
		ErrorCode:         decoded.ErrorCode,
	}, nil
}

// GetConfig returns the current configuration
func (c *Client) GetConfig() Config {
	return c.config
}
