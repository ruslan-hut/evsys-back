package redsys

import (
	"encoding/base64"
	"strings"
	"testing"
)

// testSecret is the sandbox key published in the Redsys integration guide.
const testSecret = "sq7HjrUOBfKmC576ILgskD5srU870gJ7"

func TestVerifySignature(t *testing.T) {
	const order = "3018"
	params := base64.StdEncoding.EncodeToString([]byte(
		`{"Ds_Order":"3018","Ds_Response":"0000","Ds_Merchant_Identifier":"52680a44e"}`))

	signature, err := GenerateSignature(params, testSecret, order)
	if err != nil {
		t.Fatalf("GenerateSignature: %v", err)
	}

	if err = VerifySignature(params, signature, testSecret, order); err != nil {
		t.Errorf("valid signature rejected: %v", err)
	}

	// Redsys sends notifications with the URL-safe alphabet.
	urlSafe := strings.NewReplacer("+", "-", "/", "_").Replace(signature)
	if err = VerifySignature(params, urlSafe, testSecret, order); err != nil {
		t.Errorf("url-safe signature rejected: %v", err)
	}

	// Unpadded variants are accepted too.
	if err = VerifySignature(params, strings.TrimRight(urlSafe, "="), testSecret, order); err != nil {
		t.Errorf("unpadded signature rejected: %v", err)
	}
}

func TestVerifySignatureRejects(t *testing.T) {
	const order = "3018"
	params := base64.StdEncoding.EncodeToString([]byte(
		`{"Ds_Order":"3018","Ds_Response":"0000"}`))
	signature, err := GenerateSignature(params, testSecret, order)
	if err != nil {
		t.Fatalf("GenerateSignature: %v", err)
	}

	tampered := base64.StdEncoding.EncodeToString([]byte(
		`{"Ds_Order":"3018","Ds_Response":"0000","Ds_Merchant_Identifier":"attacker"}`))

	tests := []struct {
		name   string
		params string
		sig    string
		secret string
		order  string
	}{
		{"tampered parameters", tampered, signature, testSecret, order},
		{"wrong secret", params, signature, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", order},
		{"wrong order", params, signature, testSecret, "3019"},
		{"empty signature", params, "", testSecret, order},
		{"not base64", params, "!!!not-base64!!!", testSecret, order},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifySignature(tt.params, tt.sig, tt.secret, tt.order); err == nil {
				t.Error("expected verification to fail, got nil")
			}
		})
	}
}
