package redsys

import (
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateSignature generates the HMAC SHA256 signature for Redsys REST API
// The key is diversified using 3DES-CBC with the order number
func GenerateSignature(merchantParams, secretKey, orderNumber string) (string, error) {
	// Decode the base64 secret key
	decodedKey, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret key: %w", err)
	}

	// Diversify the key using 3DES-CBC with order number
	diversifiedKey, err := diversifyKey(decodedKey, orderNumber)
	if err != nil {
		return "", fmt.Errorf("failed to diversify key: %w", err)
	}

	// Generate HMAC SHA256 signature
	h := hmac.New(sha256.New, diversifiedKey)
	h.Write([]byte(merchantParams))
	signature := h.Sum(nil)

	// Base64 encode the signature
	return base64.StdEncoding.EncodeToString(signature), nil
}

// diversifyKey uses 3DES-CBC to encrypt the order number with the secret key
// This creates a unique key for each transaction
func diversifyKey(key []byte, orderNumber string) ([]byte, error) {
	// Pad order number to 8 bytes (3DES block size)
	paddedOrder := padTo8Bytes([]byte(orderNumber))

	// Create 3DES cipher
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create 3DES cipher: %w", err)
	}

	// Use zero IV for CBC mode (Redsys specification)
	iv := make([]byte, des.BlockSize)

	// Encrypt using CBC mode
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(paddedOrder))
	mode.CryptBlocks(encrypted, paddedOrder)

	return encrypted, nil
}

// padTo8Bytes pads the input to 8 bytes using zero padding
func padTo8Bytes(data []byte) []byte {
	blockSize := 8
	padding := blockSize - (len(data) % blockSize)
	if padding == blockSize && len(data) > 0 {
		return data[:blockSize]
	}
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	if len(padded) > blockSize {
		return padded[:blockSize]
	}
	return padded
}

// EncodeBase64 encodes data to base64 URL-safe string
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes base64 string to bytes
func DecodeBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
