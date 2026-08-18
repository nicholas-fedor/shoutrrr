package pushover

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const encryptionKeySize = 32

// parseEncryptionKey decodes a 64-character hex AES-256 key.
//
// An empty key disables E2EE and returns a nil slice.
//
// Parameters:
//   - hexKey: 64-character hexadecimal string, or empty to disable encryption.
//
// Returns:
//   - key: 32-byte AES key, or nil when hexKey is empty.
//   - err: ErrInvalidEncryptionKey when hexKey is non-empty but not 32 decoded bytes.
func parseEncryptionKey(hexKey string) ([]byte, error) {
	if hexKey == "" {
		return nil, nil
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidEncryptionKey, err)
	}

	if len(key) != encryptionKeySize {
		return nil, fmt.Errorf("%w: got %d bytes, want %d", ErrInvalidEncryptionKey, len(key), encryptionKeySize)
	}

	return key, nil
}

// encryptField encrypts a single Pushover field using the documented E2EE scheme.
//
// The plaintext is gzip-compressed, encrypted with AES-256-CBC and PKCS7 padding,
// authenticated with HMAC-SHA256 over IV||ciphertext, then Base64-encoded.
//
// Parameters:
//   - plaintext: UTF-8 field value to encrypt.
//   - key: 32-byte AES-256 key.
//
// Returns:
//   - ciphertext: Base64-encoded IV||ciphertext||HMAC.
//   - err: ErrEncryptionFailed when compression, padding, or encryption fails.
func encryptField(plaintext string, key []byte) (string, error) {
	var compressed bytes.Buffer

	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("%w: gzip: %w", ErrEncryptionFailed, err)
	}

	if _, err := writer.Write([]byte(plaintext)); err != nil {
		return "", fmt.Errorf("%w: gzip: %w", ErrEncryptionFailed, err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("%w: gzip close: %w", ErrEncryptionFailed, err)
	}

	padded, err := pkcs7Pad(compressed.Bytes(), aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrEncryptionFailed, err)
	}

	initVector := make([]byte, aes.BlockSize)
	if _, err := rand.Read(initVector); err != nil {
		return "", fmt.Errorf("%w: iv: %w", ErrEncryptionFailed, err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: cipher: %w", ErrEncryptionFailed, err)
	}

	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, initVector).CryptBlocks(ciphertext, padded)

	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(initVector); err != nil {
		return "", fmt.Errorf("%w: hmac: %w", ErrEncryptionFailed, err)
	}

	if _, err := mac.Write(ciphertext); err != nil {
		return "", fmt.Errorf("%w: hmac: %w", ErrEncryptionFailed, err)
	}

	payload := make([]byte, 0, len(initVector)+len(ciphertext)+sha256.Size)
	payload = append(payload, initVector...)
	payload = append(payload, ciphertext...)
	payload = append(payload, mac.Sum(nil)...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

// pkcs7Pad appends PKCS7 padding so data is a multiple of blockSize.
//
// Parameters:
//   - data: bytes to pad.
//   - blockSize: AES block size in bytes.
//
// Returns:
//   - padded: a new slice containing data followed by PKCS7 padding.
//   - err: ErrEncryptionFailed when blockSize is invalid.
func pkcs7Pad(data []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 || blockSize > 255 {
		return nil, fmt.Errorf("%w: invalid block size %d", ErrEncryptionFailed, blockSize)
	}

	padLen := blockSize - (len(data) % blockSize)
	if padLen < 1 || padLen > 255 {
		return nil, fmt.Errorf("%w: invalid pad length %d", ErrEncryptionFailed, padLen)
	}

	padByte := byte(padLen)
	padded := make([]byte, len(data)+padLen)
	copy(padded, data)

	for i := len(data); i < len(padded); i++ {
		padded[i] = padByte
	}

	return padded, nil
}
