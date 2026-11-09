package pushover

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errInvalidCiphertext reports a field that the test decryption rejects.
var errInvalidCiphertext = errors.New("invalid ciphertext")

// decryptField reverses the Pushover end-to-end encryption scheme: it checks
// the HMAC-SHA256 over the IV and ciphertext, decrypts with AES-256-CBC,
// removes the PKCS7 padding, and decompresses the gzip data.
//
// Parameters:
//   - t: the test.
//   - encoded: the Base64-encoded IV, ciphertext, and HMAC.
//   - key: the 32-byte AES key.
//
// Returns:
//   - string: the plaintext.
//   - error: errInvalidCiphertext when the field is malformed or the HMAC does
//     not match.
func decryptField(t *testing.T, encoded string, key []byte) (string, error) {
	t.Helper()

	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(payload) < aes.BlockSize+sha256.Size {
		return "", errInvalidCiphertext
	}

	initVector := payload[:aes.BlockSize]
	ciphertext := payload[aes.BlockSize : len(payload)-sha256.Size]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(initVector)
	_, _ = mac.Write(ciphertext)

	if !hmac.Equal(payload[len(payload)-sha256.Size:], mac.Sum(nil)) || len(ciphertext)%aes.BlockSize != 0 {
		return "", errInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, initVector).CryptBlocks(plain, ciphertext)

	padLen := int(plain[len(plain)-1])
	if padLen == 0 || padLen > aes.BlockSize || !bytes.Equal(plain[len(plain)-padLen:], bytes.Repeat([]byte{byte(padLen)}, padLen)) {
		return "", errInvalidCiphertext
	}

	reader, err := gzip.NewReader(bytes.NewReader(plain[:len(plain)-padLen]))
	if err != nil {
		return "", errInvalidCiphertext
	}

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", errInvalidCiphertext
	}

	return string(decompressed), nil
}

// testKey returns the decoded test encryption key.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - []byte: the 32-byte key.
func testKey(t *testing.T) []byte {
	t.Helper()

	key, err := hex.DecodeString(testEncryptionKey)
	require.NoError(t, err)

	return key
}

// TestParseEncryptionKey verifies that an empty key disables encryption, a
// 64-character hex key of either case decodes to 32 bytes, and any other key
// is rejected.
func TestParseEncryptionKey(t *testing.T) {
	t.Parallel()

	t.Run("empty key", func(t *testing.T) {
		t.Parallel()

		key, err := parseEncryptionKey("")

		require.NoError(t, err)
		assert.Nil(t, key)
	})

	t.Run("valid keys", func(t *testing.T) {
		t.Parallel()

		for _, hexKey := range []string{testEncryptionKey, testEncryptionKeyMixed} {
			key, err := parseEncryptionKey(hexKey)

			require.NoError(t, err)
			assert.Equal(t, testKey(t), key)
		}
	})

	// invalidKeys lists keys that are not 64 hex characters.
	invalidKeys := []struct {
		name   string
		hexKey string
	}{
		{name: "short", hexKey: "0123456789abcdef"},
		{name: "long", hexKey: testEncryptionKey + "ab"},
		{name: "odd length", hexKey: testEncryptionKey[:63]},
		{name: "not hex", hexKey: strings.Repeat("z", 64)},
	}

	for _, tt := range invalidKeys {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			key, err := parseEncryptionKey(tt.hexKey)

			require.ErrorIs(t, err, ErrInvalidEncryptionKey)
			assert.Nil(t, key)
		})
	}
}

// TestEncryptField verifies that encryptField follows the documented Pushover
// scheme, so the field decrypts to its plaintext, and that each call uses a
// fresh IV.
func TestEncryptField(t *testing.T) {
	t.Parallel()

	key := testKey(t)

	for _, plaintext := range []string{"", "Secret message", strings.Repeat("long line ", 100), "ünïcødé ✓"} {
		encrypted, err := encryptField(plaintext, key)
		require.NoError(t, err)

		decrypted, err := decryptField(t, encrypted, key)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	}

	first, err := encryptField("same", key)
	require.NoError(t, err)

	second, err := encryptField("same", key)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

// TestEncryptFieldConcurrent verifies that fields encrypted at the same time
// each decrypt to their own plaintext.
//
// The gzip writers are shared through a pool, so a writer reset for one field
// must never write into another field's buffer.
func TestEncryptFieldConcurrent(t *testing.T) {
	t.Parallel()

	key := testKey(t)

	const workers, rounds = 32, 20

	// encrypted holds each worker's fields, which the test decrypts once the
	// workers are done, since require cannot stop a test from another goroutine.
	encrypted := make([][]string, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup

	for i := range workers {
		wg.Go(func() {
			for range rounds {
				field, err := encryptField(workerPlaintext(i), key)
				if err != nil {
					errs[i] = err

					return
				}

				encrypted[i] = append(encrypted[i], field)
			}
		})
	}

	wg.Wait()

	for i := range workers {
		require.NoError(t, errs[i])
		require.Len(t, encrypted[i], rounds)

		for _, field := range encrypted[i] {
			decrypted, err := decryptField(t, field, key)
			require.NoError(t, err)
			assert.Equal(t, workerPlaintext(i), decrypted)
		}
	}
}

// workerPlaintext returns the plaintext a worker of TestEncryptFieldConcurrent
// encrypts, which differs in content and length for each worker.
//
// Parameters:
//   - worker: the worker's index.
//
// Returns:
//   - string: the plaintext.
func workerPlaintext(worker int) string {
	return fmt.Sprintf("message %d %s", worker, strings.Repeat("x", worker*10))
}

// TestEncryptFieldInvalidKey verifies that encryptField fails with
// ErrEncryptionFailed for a key AES does not accept.
func TestEncryptFieldInvalidKey(t *testing.T) {
	t.Parallel()

	_, err := encryptField("message", []byte("short"))

	require.ErrorIs(t, err, ErrEncryptionFailed)
}

// TestPKCS7Pad verifies that pkcs7Pad pads to the next block boundary with
// bytes equal to the pad length, adds a full block to aligned data, and
// rejects a block size outside 1 to 255.
func TestPKCS7Pad(t *testing.T) {
	t.Parallel()

	// tests lists inputs with the padded output each should produce.
	tests := []struct {
		name      string
		data      []byte
		want      []byte
		blockSize int
	}{
		{name: "partial block", data: []byte("abc"), blockSize: 4, want: []byte("abc\x01")},
		{name: "aligned data", data: []byte("abcd"), blockSize: 4, want: []byte("abcd\x04\x04\x04\x04")},
		{name: "empty data", data: nil, blockSize: 2, want: []byte{2, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			padded, err := pkcs7Pad(tt.data, tt.blockSize)

			require.NoError(t, err)
			assert.Equal(t, tt.want, padded)
		})
	}

	for _, blockSize := range []int{0, -1, 256} {
		_, err := pkcs7Pad([]byte("abc"), blockSize)

		require.ErrorIs(t, err, ErrEncryptionFailed, "block size %d", blockSize)
	}
}
