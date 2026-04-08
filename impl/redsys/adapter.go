package redsys

import (
	"context"
	"evsys-back/impl/core"
)

// Adapter wraps the Redsys Client to implement core.RedsysClient interface
type Adapter struct {
	client *Client
}

// NewAdapter creates a new adapter for the Redsys client
func NewAdapter(client *Client) *Adapter {
	return &Adapter{client: client}
}

func toCoreResponse(resp *CaptureResponse) *core.CaptureResponse {
	return &core.CaptureResponse{
		Success:            resp.Success,
		ResponseCode:       resp.ResponseCode,
		AuthorizationCode:  resp.AuthorizationCode,
		ErrorCode:          resp.ErrorCode,
		ErrorMessage:       resp.ErrorMessage,
		MerchantIdentifier: resp.MerchantIdentifier,
		CofTxnid:           resp.CofTxnid,
		CardBrand:          resp.CardBrand,
		CardCountry:        resp.CardCountry,
		ExpiryDate:         resp.ExpiryDate,
		Order:              resp.Order,
		Amount:             resp.Amount,
		Currency:           resp.Currency,
		TransactionType:    resp.TransactionType,
		Date:               resp.Date,
		Hour:               resp.Hour,
	}
}

// Capture implements core.RedsysClient
func (a *Adapter) Capture(ctx context.Context, req core.CaptureRequest) (*core.CaptureResponse, error) {
	resp, err := a.client.Capture(ctx, CaptureRequest{
		OrderNumber:       req.OrderNumber,
		Amount:            req.Amount,
		AuthorizationCode: req.AuthorizationCode,
	})
	if err != nil {
		return nil, err
	}
	return toCoreResponse(resp), nil
}

// Cancel implements core.RedsysClient
func (a *Adapter) Cancel(ctx context.Context, req core.CaptureRequest) (*core.CaptureResponse, error) {
	resp, err := a.client.Cancel(ctx, CaptureRequest{
		OrderNumber:       req.OrderNumber,
		Amount:            req.Amount,
		AuthorizationCode: req.AuthorizationCode,
	})
	if err != nil {
		return nil, err
	}
	return toCoreResponse(resp), nil
}

// Preauthorize implements core.RedsysClient
func (a *Adapter) Preauthorize(ctx context.Context, req core.PreauthorizeRequest) (*core.CaptureResponse, error) {
	resp, err := a.client.Preauthorize(ctx, PreauthorizeRequest{
		OrderNumber: req.OrderNumber,
		Amount:      req.Amount,
		CardToken:   req.CardToken,
		CofTid:      req.CofTid,
	})
	if err != nil {
		return nil, err
	}
	return toCoreResponse(resp), nil
}

// Pay implements core.RedsysClient
func (a *Adapter) Pay(ctx context.Context, req core.PayRequest) (*core.CaptureResponse, error) {
	resp, err := a.client.Pay(ctx, PayRequest{
		OrderNumber: req.OrderNumber,
		Amount:      req.Amount,
		CardToken:   req.CardToken,
		CofTid:      req.CofTid,
	})
	if err != nil {
		return nil, err
	}
	return toCoreResponse(resp), nil
}

// Refund implements core.RedsysClient
func (a *Adapter) Refund(ctx context.Context, req core.RefundRequest) (*core.CaptureResponse, error) {
	resp, err := a.client.Refund(ctx, RefundRequest{
		OrderNumber: req.OrderNumber,
		Amount:      req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return toCoreResponse(resp), nil
}

// Tokenize implements core.RedsysClient. It performs a Customer-Initiated
// authorization against Redsys REST using the temporary idOper returned
// by the inSite JS SDK, and extracts the permanent card token from the
// response.
func (a *Adapter) Tokenize(ctx context.Context, req core.TokenizeRequest) (*core.TokenizeResponse, error) {
	resp, err := a.client.Tokenize(ctx, TokenizeRequest{
		OrderNumber: req.OrderNumber,
		IdOper:      req.IdOper,
		Amount:      req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return &core.TokenizeResponse{
		Success:           resp.Success,
		ResponseCode:      resp.ResponseCode,
		ErrorCode:         resp.ErrorCode,
		ErrorMessage:      resp.ErrorMessage,
		CardIdentifier:    resp.CardIdentifier,
		CofTxnid:          resp.CofTxnid,
		CardBrand:         resp.CardBrand,
		CardCountry:       resp.CardCountry,
		CardType:          resp.CardType,
		ExpiryDate:        resp.ExpiryDate,
		AuthorizationCode: resp.AuthorizationCode,
	}, nil
}

// MerchantCode implements core.RedsysClient. Exposes the configured FUC
// so Core.CreateInSiteOrder can hand it to the Angular frontend without
// duplicating configuration across layers.
func (a *Adapter) MerchantCode() string { return a.client.GetConfig().MerchantCode }

// Terminal implements core.RedsysClient.
func (a *Adapter) Terminal() string { return a.client.GetConfig().Terminal }
