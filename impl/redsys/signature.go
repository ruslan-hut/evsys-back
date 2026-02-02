package redsys

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateSignature generates the HMAC SHA256 signature for Redsys REST API.
// The signature process:
// 1. Decode the merchant secret from Base64
// 2. Encrypt the order number using 3DES-CBC with zero-padding (Redsys requirement)
// 3. Use the encrypted result as HMAC key to sign the parameters
// 4. Return the Base64-encoded HMAC signature
func GenerateSignature(merchantParams, secretKey, orderNumber string) (string, error) {
	// Decode the base64 secret key
	decodedKey, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret key: %w", err)
	}

	// Encrypt order number with 3DES to create diversified key
	diversifiedKey, err := encrypt3DES(orderNumber, decodedKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt order: %w", err)
	}

	// Generate HMAC SHA256 signature using encrypted order as key
	h := hmac.New(sha256.New, diversifiedKey)
	h.Write([]byte(merchantParams))
	signature := h.Sum(nil)

	// Base64 encode the signature
	return base64.StdEncoding.EncodeToString(signature), nil
}

// encrypt3DES encrypts plaintext using 3DES in CBC mode with zero-padding.
// Redsys-specific requirements (mandated by their API specification):
// 1. Fixed all-zero IV (not cryptographically secure but required)
// 2. Zero-padding (NOT PKCS#7 - this is critical for signature verification)
func encrypt3DES(plainText string, key []byte) ([]byte, error) {
	if plainText == "" {
		return nil, fmt.Errorf("plainText cannot be empty")
	}

	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create 3DES cipher: %w", err)
	}

	// Fixed IV as required by Redsys specification
	iv := []byte{0, 0, 0, 0, 0, 0, 0, 0}

	// Apply zero-padding as required by Redsys signature algorithm
	// NOTE: Redsys expects zero-padding, NOT PKCS#7 padding
	toEncrypt := zeroPad([]byte(plainText), block.BlockSize())

	ciphertext := make([]byte, len(toEncrypt))

	// Encrypt using CBC mode with fixed IV
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, toEncrypt)

	return ciphertext, nil
}

// zeroPad applies zero-byte padding to make data a multiple of blockSize.
// This is specifically required by Redsys for signature calculation.
func zeroPad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	if padding == blockSize {
		// Already aligned, no padding needed
		return data
	}
	padText := bytes.Repeat([]byte{0x00}, padding)
	return append(data, padText...)
}
