package aireport

import (
	"strings"
	"testing"
)

func TestTemplateOf(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "connection reset keeps the shape, drops the addresses",
			text: "read error: read tcp 217.112.80.212:9000->95.61.67.36:52818: read: connection reset by peer",
			want: "read error: read tcp <addr>: read: connection reset by peer",
		},
		{
			name: "vendor error codes survive",
			text: "connector #1: Faulted (OtherError; CO.30.003)",
			want: "connector #N: Faulted (OtherError; CO.30.003)",
		},
		{
			name: "connection counter",
			text: "registered new connection: total connections 10",
			want: "registered new connection: total connections N",
		},
		{
			name: "user ids are removed",
			text: "enrollment: failed to save card for user user_158c0: payment method with identifier 026d0af1ab... already exists",
			want: "enrollment: failed to save card for user user_*: payment method with identifier <id> already exists",
		},
		{
			name: "durations and times",
			text: "transaction #5223 closed by sweep: no activity from a suspended connector (idle 6h1m40s, last activity 2026-10-25T12:10:34Z)",
			want: "transaction #N closed by sweep: no activity from a suspended connector (idle <dur>, last activity <time>)",
		},
		{
			name: "emails are removed",
			text: "warning mail to ops@example.com failed",
			want: "warning mail to <email> failed",
		},
		{
			name: "uppercase rfid tags",
			text: "Authorize 04A1B2C3D4E5: Accepted",
			want: "Authorize <id>: Accepted",
		},
		{
			name: "identifying key=value pairs",
			text: "authorize idTag=JOHNSMITH user=maria status=Accepted",
			want: "authorize idTag=<id> user=<id> status=Accepted",
		},
		{
			name: "long text is cut on a rune boundary",
			text: strings.Repeat("ñ", 150),
			want: strings.Repeat("ñ", 100) + "…",
		},
		{
			name: "decimal amounts",
			text: "transaction 5208: payment failed on order 3185 (SIS0334); next retry attempt 4",
			want: "transaction N: payment failed on order N (SIS0334); next retry attempt N",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := templateOf(tt.text); got != tt.want {
				t.Errorf("templateOf()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestTemplateOfGroupsSameShape(t *testing.T) {
	a := templateOf("read error: read tcp 217.112.80.212:9000->87.247.129.133:39818: read: connection reset by peer")
	b := templateOf("read error: read tcp 217.112.80.212:9000->87.247.129.133:54764: read: connection reset by peer")
	if a != b {
		t.Errorf("same event, different shapes: %q, %q", a, b)
	}
}
