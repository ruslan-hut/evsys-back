package openai

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const completedBody = `{
	"id": "resp_1", "model": "gpt-6.1-sol", "status": "completed",
	"output": [
		{"type": "reasoning", "summary": []},
		{"type": "message", "content": [{"type": "output_text", "text": "{\"ok\":true}"}]}
	],
	"usage": {
		"input_tokens": 1200, "input_tokens_details": {"cached_tokens": 1024},
		"output_tokens": 300, "output_tokens_details": {"reasoning_tokens": 250}
	}
}`

func TestRespond(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantText string
		wantErr  string
		// usage must survive an incomplete answer: it is paid for
		wantInput int64
	}{
		{name: "completed", status: 200, body: completedBody, wantText: `{"ok":true}`, wantInput: 1200},
		{
			name:   "cut at the token limit",
			status: 200,
			body: `{"id":"resp_2","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[],
				"usage":{"input_tokens":900,"output_tokens":1000,"output_tokens_details":{"reasoning_tokens":1000}}}`,
			wantErr:   "max_output_tokens",
			wantInput: 900,
		},
		{
			name:      "refusal",
			status:    200,
			body:      `{"id":"resp_3","status":"completed","output":[{"type":"message","content":[{"type":"refusal","refusal":"no"}]}],"usage":{"input_tokens":10,"output_tokens":1}}`,
			wantErr:   "refused",
			wantInput: 10,
		},
		{
			name:    "api error",
			status:  401,
			body:    `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error","code":"invalid_api_key"}}`,
			wantErr: "Incorrect API key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer key" {
					t.Errorf("request %s %s", r.URL.Path, r.Header.Get("Authorization"))
				}
				_ = json.NewDecoder(r.Body).Decode(&got)
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			c := New(Config{ApiKey: "key", ApiUrl: srv.URL + "/v1/"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			resp, err := c.Respond(context.Background(), Request{
				Model:           "gpt-6.1-sol",
				Instructions:    "be brief",
				Input:           "hi",
				SchemaName:      "test",
				Schema:          json.RawMessage(`{"type":"object"}`),
				MaxOutputTokens: 1000,
				ReasoningEffort: "low",
			})

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tt.wantInput > 0 && (resp == nil || resp.Usage.InputTokens != tt.wantInput) {
				t.Errorf("usage lost: %+v", resp)
			}
			if tt.wantText != "" {
				if resp.Text != tt.wantText {
					t.Errorf("text = %q", resp.Text)
				}
				if resp.Usage.CachedTokens != 1024 || resp.Usage.ReasoningTokens != 250 {
					t.Errorf("usage = %+v", resp.Usage)
				}
			}

			if got["store"] != false {
				t.Error("responses must not be stored at OpenAI")
			}
			format := got["text"].(map[string]any)["format"].(map[string]any)
			if format["type"] != "json_schema" || format["strict"] != true || format["name"] != "test" {
				t.Errorf("format = %v", format)
			}
			if got["reasoning"].(map[string]any)["effort"] != "low" {
				t.Errorf("reasoning = %v", got["reasoning"])
			}
		})
	}
}

func TestRespondNotConfigured(t *testing.T) {
	c := New(Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if c.Configured() {
		t.Fatal("no key, not configured")
	}
	if _, err := c.Respond(context.Background(), Request{}); err != ErrNotConfigured {
		t.Errorf("err = %v", err)
	}
}
