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
	TransactionTypePay          = "0" // Authorization/Purchase (direct MIT payment)
	TransactionTypePreauthorize = "1"
	TransactionTypeCapture      = "2"
	TransactionTypeRefund       = "3"
	TransactionTypeCancel       = "9"

	// Response codes
	ResponseCodeOK       = "0000"
	ResponseCodeRefundOK = "0900"
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
	ResponseCode       string `json:"Ds_Response"`
	AuthorisationCode  string `json:"Ds_AuthorisationCode"`
	Order              string `json:"Ds_Order"`
	Amount             string `json:"Ds_Amount"`
	Currency           string `json:"Ds_Currency"`
	ErrorCode          string `json:"Ds_ErrorCode,omitempty"`
	MerchantIdentifier string `json:"Ds_Merchant_Identifier,omitempty"`
	MerchantCofTxnid   string `json:"Ds_Merchant_Cof_Txnid,omitempty"`
	CardBrand          string `json:"Ds_Card_Brand,omitempty"`
	CardCountry        string `json:"Ds_Card_Country,omitempty"`
	ExpiryDate         string `json:"Ds_ExpiryDate,omitempty"`
	TransactionType    string `json:"Ds_TransactionType,omitempty"`
	Date               string `json:"Ds_Date,omitempty"`
	Hour               string `json:"Ds_Hour,omitempty"`
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
	// Extended fields from decoded Redsys response
	MerchantIdentifier string
	CofTxnid           string
	CardBrand          string
	CardCountry        string
	ExpiryDate         string
	Order              string
	Amount             string
	Currency           string
	TransactionType    string
	Date               string
	Hour               string
}

// PreauthorizeRequest represents a preauthorization request
type PreauthorizeRequest struct {
	OrderNumber string
	Amount      int
	CardToken   string // DS_MERCHANT_IDENTIFIER (card reference from Redsys)
	CofTid      string // DS_MERCHANT_COF_TXNID (network transaction ID from initial auth)
}

// PayRequest represents a direct MIT payment request (transaction type "0")
type PayRequest struct {
	OrderNumber string
	Amount      int
	CardToken   string
	CofTid      string
}

// RefundRequest represents a refund request (transaction type "3")
type RefundRequest struct {
	OrderNumber string
	Amount      int
}

// Capture performs a capture operation on a preauthorized amount
func (c *Client) Capture(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	return c.performCaptureTransaction(ctx, req, TransactionTypeCapture)
}

// Cancel performs a cancellation of a preauthorization
func (c *Client) Cancel(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	return c.performCaptureTransaction(ctx, req, TransactionTypeCancel)
}

// Preauthorize performs a MIT preauthorization with a saved card token
func (c *Client) Preauthorize(ctx context.Context, req PreauthorizeRequest) (*CaptureResponse, error) {
	return c.performMITTransaction(ctx, req.OrderNumber, req.Amount, req.CardToken, req.CofTid, TransactionTypePreauthorize)
}

// Pay performs a direct MIT payment with a saved card token (transaction type "0")
func (c *Client) Pay(ctx context.Context, req PayRequest) (*CaptureResponse, error) {
	return c.performMITTransaction(ctx, req.OrderNumber, req.Amount, req.CardToken, req.CofTid, TransactionTypePay)
}

// Refund performs a refund for a given order (transaction type "3")
func (c *Client) Refund(ctx context.Context, req RefundRequest) (*CaptureResponse, error) {
	return c.performSimpleTransaction(ctx, req.OrderNumber, req.Amount, TransactionTypeRefund)
}

// performMITTransaction executes a Merchant Initiated Transaction with stored credentials.
// Used by both Preauthorize (type "1") and Pay (type "0").
func (c *Client) performMITTransaction(ctx context.Context, orderNumber string, amount int, cardToken, cofTid, txType string) (*CaptureResponse, error) {
	log := c.log.With(
		slog.String("order", orderNumber),
		slog.Int("amount", amount),
		slog.String("tx_type", txType),
	)

	params := MerchantParameters{
		MerchantCode:    c.config.MerchantCode,
		Terminal:        c.config.Terminal,
		TransactionType: txType,
		Amount:          fmt.Sprintf("%d", amount),
		Currency:        c.config.Currency,
		Order:           orderNumber,
		Identifier:      cardToken,
		DirectPayment:   "true",
		Exception:       "MIT",
		CofIni:          "N",
		CofType:         "R",
		CofTid:          cofTid,
	}

	return c.sendRequest(ctx, log, params, orderNumber)
}

// performSimpleTransaction executes a transaction without MIT/COF fields (capture, cancel, refund).
func (c *Client) performSimpleTransaction(ctx context.Context, orderNumber string, amount int, txType string) (*CaptureResponse, error) {
	log := c.log.With(
		slog.String("order", orderNumber),
		slog.Int("amount", amount),
		slog.String("tx_type", txType),
	)

	params := MerchantParameters{
		MerchantCode:    c.config.MerchantCode,
		Terminal:        c.config.Terminal,
		TransactionType: txType,
		Amount:          fmt.Sprintf("%d", amount),
		Currency:        c.config.Currency,
		Order:           orderNumber,
	}

	return c.sendRequest(ctx, log, params, orderNumber)
}

// sendRequest marshals parameters, signs, sends HTTP request, and decodes the response.
func (c *Client) sendRequest(ctx context.Context, log *slog.Logger, params MerchantParameters, orderNumber string) (*CaptureResponse, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}
	merchantParams := base64.StdEncoding.EncodeToString(paramsJSON)

	signature, err := GenerateSignature(merchantParams, c.config.SecretKey, orderNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signature: %w", err)
	}

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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.With(
		slog.Int("status_code", resp.StatusCode),
		slog.String("body", string(respBody)),
	).Debug("Redsys response received")

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

	return c.decodeResponse(log, respBody)
}

// decodeResponse parses the Redsys REST API response body into a CaptureResponse.
func (c *Client) decodeResponse(log *slog.Logger, body []byte) (*CaptureResponse, error) {
	var apiResp Response
	if err := json.Unmarshal(body, &apiResp); err != nil {
		var errResp ErrorCodeResponse
		if errErr := json.Unmarshal(body, &errResp); errErr == nil && errResp.Code != "" {
			log.With(slog.String("error_code", errResp.Code)).Warn("Redsys returned error code")
			return &CaptureResponse{
				Success:      false,
				ErrorCode:    errResp.Code,
				ErrorMessage: fmt.Sprintf("Redsys error: %s", errResp.Code),
			}, nil
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.MerchantParameters == "" {
		var errResp ErrorCodeResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Code != "" {
			log.With(slog.String("error_code", errResp.Code)).Warn("Redsys returned error code")
			return &CaptureResponse{
				Success:      false,
				ErrorCode:    errResp.Code,
				ErrorMessage: fmt.Sprintf("Redsys error: %s", errResp.Code),
			}, nil
		}
		log.With(slog.String("body", string(body))).Warn("Redsys returned empty merchant parameters")
		return &CaptureResponse{
			Success:      false,
			ErrorCode:    "EMPTY_RESPONSE",
			ErrorMessage: "Redsys returned empty merchant parameters",
		}, nil
	}

	decodedParams, err := base64.StdEncoding.DecodeString(apiResp.MerchantParameters)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response parameters: %w", err)
	}

	var decoded DecodedResponse
	if err := json.Unmarshal(decodedParams, &decoded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decoded parameters: %w", err)
	}

	success := decoded.ResponseCode == ResponseCodeOK || (len(decoded.ResponseCode) == 4 && decoded.ResponseCode[0] == '0')

	log.With(
		slog.String("response_code", decoded.ResponseCode),
		slog.Bool("success", success),
	).Info("Redsys transaction completed")

	return &CaptureResponse{
		Success:            success,
		ResponseCode:       decoded.ResponseCode,
		AuthorizationCode:  decoded.AuthorisationCode,
		ErrorCode:          decoded.ErrorCode,
		MerchantIdentifier: decoded.MerchantIdentifier,
		CofTxnid:           decoded.MerchantCofTxnid,
		CardBrand:          decoded.CardBrand,
		CardCountry:        decoded.CardCountry,
		ExpiryDate:         decoded.ExpiryDate,
		Order:              decoded.Order,
		Amount:             decoded.Amount,
		Currency:           decoded.Currency,
		TransactionType:    decoded.TransactionType,
		Date:               decoded.Date,
		Hour:               decoded.Hour,
	}, nil
}

// performCaptureTransaction executes a capture/cancel transaction with authorization code.
func (c *Client) performCaptureTransaction(ctx context.Context, req CaptureRequest, txType string) (*CaptureResponse, error) {
	log := c.log.With(
		slog.String("order", req.OrderNumber),
		slog.Int("amount", req.Amount),
		slog.String("tx_type", txType),
	)

	params := MerchantParameters{
		MerchantCode:      c.config.MerchantCode,
		Terminal:          c.config.Terminal,
		TransactionType:   txType,
		Amount:            fmt.Sprintf("%d", req.Amount),
		Currency:          c.config.Currency,
		Order:             req.OrderNumber,
		AuthorisationCode: req.AuthorizationCode,
	}

	return c.sendRequest(ctx, log, params, req.OrderNumber)
}

// GetConfig returns the current configuration
func (c *Client) GetConfig() Config {
	return c.config
}
