package brevo

import (
	"bytes"
	"context"
	"encoding/json"
	"evsys-back/internal/lib/sl"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Config holds the Brevo (Sendinblue) transactional email configuration.
type Config struct {
	ApiKey     string
	SenderName string
	SenderMail string
	ApiUrl     string
}

// Client posts transactional emails to the Brevo HTTP API.
type Client struct {
	cfg  Config
	http *http.Client
	log  *slog.Logger
}

func New(cfg Config, log *slog.Logger) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 15 * time.Second},
		log:  log.With(sl.Module("brevo.client")),
	}
}

type address struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendRequest struct {
	Sender      address   `json:"sender"`
	To          []address `json:"to"`
	Subject     string    `json:"subject"`
	HtmlContent string    `json:"htmlContent"`
}

// Send posts an HTML email to a single recipient. Errors include the response
// body to make Brevo-side validation failures debuggable.
func (c *Client) Send(ctx context.Context, to, subject, htmlBody string) error {
	body, err := json.Marshal(sendRequest{
		Sender:      address{Email: c.cfg.SenderMail, Name: c.cfg.SenderName},
		To:          []address{{Email: to}},
		Subject:     subject,
		HtmlContent: htmlBody,
	})
	if err != nil {
		return fmt.Errorf("marshal brevo request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.ApiUrl, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build brevo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", c.cfg.ApiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("brevo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.log.Debug("brevo email sent", slog.String("to", to), slog.Int("status", resp.StatusCode))
		return nil
	}
	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("brevo status %d: %s", resp.StatusCode, string(respBody))
}
