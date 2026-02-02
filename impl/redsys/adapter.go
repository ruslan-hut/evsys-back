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

// Capture implements core.RedsysClient
func (a *Adapter) Capture(ctx context.Context, req core.CaptureRequest) (*core.CaptureResponse, error) {
	redsysReq := CaptureRequest{
		OrderNumber:       req.OrderNumber,
		Amount:            req.Amount,
		AuthorizationCode: req.AuthorizationCode,
	}

	resp, err := a.client.Capture(ctx, redsysReq)
	if err != nil {
		return nil, err
	}

	return &core.CaptureResponse{
		Success:           resp.Success,
		ResponseCode:      resp.ResponseCode,
		AuthorizationCode: resp.AuthorizationCode,
		ErrorCode:         resp.ErrorCode,
		ErrorMessage:      resp.ErrorMessage,
	}, nil
}

// Cancel implements core.RedsysClient
func (a *Adapter) Cancel(ctx context.Context, req core.CaptureRequest) (*core.CaptureResponse, error) {
	redsysReq := CaptureRequest{
		OrderNumber:       req.OrderNumber,
		Amount:            req.Amount,
		AuthorizationCode: req.AuthorizationCode,
	}

	resp, err := a.client.Cancel(ctx, redsysReq)
	if err != nil {
		return nil, err
	}

	return &core.CaptureResponse{
		Success:           resp.Success,
		ResponseCode:      resp.ResponseCode,
		AuthorizationCode: resp.AuthorizationCode,
		ErrorCode:         resp.ErrorCode,
		ErrorMessage:      resp.ErrorMessage,
	}, nil
}
