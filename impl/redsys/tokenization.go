package redsys

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// InSiteTokenizationRequest is the input for BuildTokenizationParams.
// The order number must already be normalized to 12 digits.
type InSiteTokenizationRequest struct {
	OrderNumber string
	Amount      int    // amount in cents for the authorization/verification
	Description string // optional product description echoed to Redsys
}

// InSiteTokenizationParams holds the pre-signed Redsys parameters the browser
// must hand to the inSite JS SDK in order to tokenize a card on the merchant
// page. The merchant's public identifiers (MerchantCode + Terminal) are
// returned too so the frontend can initialize the inSite widget without
// duplicating configuration.
type InSiteTokenizationParams struct {
	SignatureVersion   string
	MerchantParameters string // base64-encoded JSON of MerchantParameters
	Signature          string // base64-encoded HMAC-SHA256
	MerchantCode       string
	Terminal           string
	OrderNumber        string
}

// BuildTokenizationParams builds the signed Ds_* triplet for an inSite card
// tokenization call. Redsys tokenizes a card by running a low-value
// authorization (transaction type "0") with DS_MERCHANT_IDENTIFIER="REQUIRED"
// and DS_MERCHANT_COF_INI="S" — the response carries back the card
// reference (Ds_Merchant_Identifier) and the initial COF transaction id
// (Ds_Merchant_Cof_Txnid) that future MIT charges need.
//
// No HTTP request is made here; the merchant server only signs the payload.
// The actual authorization happens in the browser via the inSite JS SDK.
func (c *Client) BuildTokenizationParams(req InSiteTokenizationRequest) (*InSiteTokenizationParams, error) {
	if req.OrderNumber == "" {
		return nil, fmt.Errorf("order number is required")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	params := MerchantParameters{
		MerchantCode:    c.config.MerchantCode,
		Terminal:        c.config.Terminal,
		TransactionType: TransactionTypePay, // "0" — authorization
		Amount:          fmt.Sprintf("%d", req.Amount),
		Currency:        c.config.Currency,
		Order:           req.OrderNumber,
		Identifier:      "REQUIRED", // ask Redsys to issue a card token
		CofIni:          "S",        // start of a Credential-on-File chain
		CofType:         "R",        // recurring usage (EV charging sessions)
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal merchant parameters: %w", err)
	}
	merchantParams := base64.StdEncoding.EncodeToString(paramsJSON)

	signature, err := GenerateSignature(merchantParams, c.config.SecretKey, req.OrderNumber)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	return &InSiteTokenizationParams{
		SignatureVersion:   "HMAC_SHA256_V1",
		MerchantParameters: merchantParams,
		Signature:          signature,
		MerchantCode:       c.config.MerchantCode,
		Terminal:           c.config.Terminal,
		OrderNumber:        req.OrderNumber,
	}, nil
}
