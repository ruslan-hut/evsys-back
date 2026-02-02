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
	TransactionTypePreauthorize = "1"
	TransactionTypeCapture      = "2"
	TransactionTypeCancel       = "9"

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
	MerchantCode      string `json:"DS_MERCHANT_MERCHANTCODE"`
	Terminal          string `json:"DS_MERCHANT_TERMINAL"`
	TransactionType   string `json:"DS_MERCHANT_TRANSACTIONTYPE"`
	Amount            string `json:"DS_MERCHANT_AMOUNT"`
	Currency          string `json:"DS_MERCHANT_CURRENCY"`
	Order             string `json:"DS_MERCHANT_ORDER"`
	AuthorisationCode string `json:"DS_MERCHANT_AUTHORISATIONCODE,omitempty"`
	Identifier        string `json:"DS_MERCHANT_IDENTIFIER,omitempty"`
	DirectPayment     string `json:"DS_MERCHANT_DIRECTPAYMENT,omitempty"`
	// MIT (Merchant Initiated Transaction) / PSD2 fields
	Exception string `json:"DS_MERCHANT_EXCEP_SCA,omitempty"`
	CofIni    string `json:"DS_MERCHANT_COF_INI,omitempty"`
	CofType   string `json:"DS_MERCHANT_COF_TYPE,omitempty"`
	CofTid    string `json:"DS_MERCHANT_COF_TXNID,omitempty"`
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

// ErrorCodeResponse represents an error response from Redsys
type ErrorCodeResponse struct {
	Code string `json:"errorCode"`
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

// PreauthorizeRequest represents a preauthorization request
type PreauthorizeRequest struct {
	OrderNumber string
	Amount      int
	CardToken   string // DS_MERCHANT_IDENTIFIER (card reference from Redsys)
	CofTid      string // DS_MERCHANT_COF_TXNID (network transaction ID from initial auth)
}

// Capture performs a capture operation on a preauthorized amount
func (c *Client) Capture(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	return c.performTransaction(ctx, req, TransactionTypeCapture)
}

// Cancel performs a cancellation of a preauthorization
func (c *Client) Cancel(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	return c.performTransaction(ctx, req, TransactionTypeCancel)
}

// Preauthorize performs a MIT preauthorization with a saved card token
func (c *Client) Preauthorize(ctx context.Context, req PreauthorizeRequest) (*CaptureResponse, error) {
	log := c.log.With(
		slog.String("order", req.OrderNumber),
		slog.Int("amount", req.Amount),
		slog.String("tx_type", TransactionTypePreauthorize),
	)

	// Build merchant parameters for MIT preauthorization
	// Uses PSD2 MIT exemption for merchant-initiated transactions with stored credentials
	params := MerchantParameters{
		MerchantCode:    c.config.MerchantCode,
		Terminal:        c.config.Terminal,
		TransactionType: TransactionTypePreauthorize,
		Amount:          fmt.Sprintf("%d", req.Amount),
		Currency:        c.config.Currency,
		Order:           req.OrderNumber,
		Identifier:      req.CardToken,
		DirectPayment:   "true",
		// MIT/PSD2 parameters for merchant-initiated transactions
		Exception: "MIT", // PSD2 Merchant Initiated Transaction exemption
		CofIni:    "N",   // N = subsequent use of stored credentials (not initial)
		CofType:   "R",   // R = Recurring (variable amounts, EV charging)
		CofTid:    req.CofTid,
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

	log.Debug("sending preauthorization request to Redsys")

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

	log.With(
		slog.Int("status_code", resp.StatusCode),
		slog.String("body", string(respBody)),
	).Debug("Redsys preauthorization response received")

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

	// Parse response - first try normal response format
	var apiResp Response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		// Try to parse as error response
		var errResp ErrorCodeResponse
		if errErr := json.Unmarshal(respBody, &errResp); errErr == nil && errResp.Code != "" {
			log.With(slog.String("error_code", errResp.Code)).Warn("Redsys returned error code")
			return &CaptureResponse{
				Success:      false,
				ErrorCode:    errResp.Code,
				ErrorMessage: fmt.Sprintf("Redsys error: %s", errResp.Code),
			}, nil
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check if we got an error response (no merchant parameters)
	if apiResp.MerchantParameters == "" {
		// Try to parse as error response
		var errResp ErrorCodeResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Code != "" {
			log.With(slog.String("error_code", errResp.Code)).Warn("Redsys returned error code")
			return &CaptureResponse{
				Success:      false,
				ErrorCode:    errResp.Code,
				ErrorMessage: fmt.Sprintf("Redsys error: %s", errResp.Code),
			}, nil
		}
		log.With(slog.String("body", string(respBody))).Warn("Redsys returned empty merchant parameters")
		return &CaptureResponse{
			Success:      false,
			ErrorCode:    "EMPTY_RESPONSE",
			ErrorMessage: "Redsys returned empty merchant parameters",
		}, nil
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

	// Check response code (0000-0099 are success codes)
	success := decoded.ResponseCode == ResponseCodeOK || (len(decoded.ResponseCode) == 4 && decoded.ResponseCode[0] == '0')

	log.With(
		slog.String("response_code", decoded.ResponseCode),
		slog.Bool("success", success),
	).Info("Redsys preauthorization completed")

	return &CaptureResponse{
		Success:           success,
		ResponseCode:      decoded.ResponseCode,
		AuthorizationCode: decoded.AuthorisationCode,
		ErrorCode:         decoded.ErrorCode,
	}, nil
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
