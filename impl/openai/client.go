// Package openai is a minimal client of the OpenAI Responses API: one request,
// a strict JSON schema for the answer, and the token usage it cost.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"evsys-back/internal/lib/sl"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultApiUrl is the OpenAI API base used when the configuration names none.
const DefaultApiUrl = "https://api.openai.com/v1"

// maxResponseBytes bounds the response read into memory; an answer within
// the token limits is far smaller.
const maxResponseBytes = 8 << 20

// ErrNotConfigured means the service configuration carries no API key.
var ErrNotConfigured = errors.New("openai api key not configured")

// Config holds the OpenAI connection settings.
type Config struct {
	ApiKey string
	ApiUrl string
	// Timeout bounds one request; reasoning models take minutes on a large
	// prompt.
	Timeout time.Duration
}

// Client posts requests to the Responses API.
type Client struct {
	cfg  Config
	http *http.Client
	log  *slog.Logger
}

// New builds a client; without an API key it is built but not Configured.
func New(cfg Config, log *slog.Logger) *Client {
	if cfg.ApiUrl == "" {
		cfg.ApiUrl = DefaultApiUrl
	}
	cfg.ApiUrl = strings.TrimRight(cfg.ApiUrl, "/")
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
		log:  log.With(sl.Module("impl.openai")),
	}
}

// Configured reports whether the client has an API key.
func (c *Client) Configured() bool {
	return c != nil && c.cfg.ApiKey != ""
}

// Request is one call. Instructions go first so that requests sharing them and
// the start of Input hit OpenAI's prompt cache.
type Request struct {
	Model           string
	Instructions    string
	Input           string
	SchemaName      string
	Schema          json.RawMessage
	MaxOutputTokens int
	ReasoningEffort string
}

// Usage is what a call cost, in the API's own accounting. Output includes the
// reasoning tokens.
type Usage struct {
	InputTokens     int64
	CachedTokens    int64
	OutputTokens    int64
	ReasoningTokens int64
}

// Response is the answer text (JSON when a schema was given) with its usage.
// Usage is filled whenever the API reported it, also on an incomplete answer,
// so that the caller can account for a failed call too.
type Response struct {
	Id     string
	Model  string
	Text   string
	Usage  Usage
	Status string
}

type apiRequest struct {
	Model           string         `json:"model"`
	Instructions    string         `json:"instructions,omitempty"`
	Input           string         `json:"input"`
	MaxOutputTokens int            `json:"max_output_tokens,omitempty"`
	Reasoning       *apiReasoning  `json:"reasoning,omitempty"`
	Text            *apiTextFormat `json:"text,omitempty"`
	Store           bool           `json:"store"`
}

type apiReasoning struct {
	Effort string `json:"effort"`
}

type apiTextFormat struct {
	Format apiFormat `json:"format"`
}

type apiFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type apiResponse struct {
	Id                string `json:"id"`
	Model             string `json:"model"`
	Status            string `json:"status"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error  *apiError `json:"error"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	} `json:"output"`
	Usage *struct {
		InputTokens        int64 `json:"input_tokens"`
		InputTokensDetails struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		OutputTokens        int64 `json:"output_tokens"`
		OutputTokensDetails struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type apiErrorEnvelope struct {
	Error *apiError `json:"error"`
}

// Respond makes one request. An answer that is not complete - cut at the token
// limit, refused, failed - is an error, returned together with the response
// so that its usage is not lost.
func (c *Client) Respond(ctx context.Context, req Request) (*Response, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	payload := apiRequest{
		Model:           req.Model,
		Instructions:    req.Instructions,
		Input:           req.Input,
		MaxOutputTokens: req.MaxOutputTokens,
	}
	if req.ReasoningEffort != "" {
		payload.Reasoning = &apiReasoning{Effort: req.ReasoningEffort}
	}
	if len(req.Schema) > 0 {
		payload.Text = &apiTextFormat{Format: apiFormat{
			Type:   "json_schema",
			Name:   req.SchemaName,
			Schema: req.Schema,
			Strict: true,
		}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.ApiUrl+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.ApiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read openai response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env apiErrorEnvelope
		if json.Unmarshal(respBody, &env) == nil && env.Error != nil && env.Error.Message != "" {
			return nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, env.Error.Message)
		}
		return nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var parsed apiResponse
	if err = json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}

	out := &Response{Id: parsed.Id, Model: parsed.Model, Status: parsed.Status}
	if parsed.Usage != nil {
		out.Usage = Usage{
			InputTokens:     parsed.Usage.InputTokens,
			CachedTokens:    parsed.Usage.InputTokensDetails.CachedTokens,
			OutputTokens:    parsed.Usage.OutputTokens,
			ReasoningTokens: parsed.Usage.OutputTokensDetails.ReasoningTokens,
		}
	}
	c.log.With(
		slog.String("id", out.Id),
		slog.String("model", out.Model),
		slog.String("status", out.Status),
		slog.Int64("input_tokens", out.Usage.InputTokens),
		slog.Int64("cached_tokens", out.Usage.CachedTokens),
		slog.Int64("output_tokens", out.Usage.OutputTokens),
		slog.Int64("reasoning_tokens", out.Usage.ReasoningTokens),
	).Info("openai response")

	if parsed.Error != nil && parsed.Error.Message != "" {
		return out, fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if parsed.Status != "completed" {
		reason := parsed.Status
		if parsed.IncompleteDetails != nil && parsed.IncompleteDetails.Reason != "" {
			reason += ": " + parsed.IncompleteDetails.Reason
		}
		return out, fmt.Errorf("openai answer not complete (%s)", reason)
	}

	var text strings.Builder
	for _, item := range parsed.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			switch part.Type {
			case "output_text":
				text.WriteString(part.Text)
			case "refusal":
				return out, fmt.Errorf("openai refused: %s", part.Refusal)
			}
		}
	}
	out.Text = text.String()
	if out.Text == "" {
		return out, errors.New("openai answer has no text")
	}
	return out, nil
}

// truncate cuts s to at most n bytes on a rune boundary: the text ends up in
// MongoDB, which refuses invalid UTF-8.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
