package pushover

import (
	"strings"
	"testing"
)

// BenchmarkEncryptField measures encrypting one field for end-to-end
// encryption.
//
// A send with an encryption key encrypts its message and its title, so each
// costs one call. The short field is a typical title, and the long one fills
// Pushover's 1024-character message limit.
func BenchmarkEncryptField(b *testing.B) {
	key, err := parseEncryptionKey(strings.Repeat("ab", encryptionKeySize))
	if err != nil {
		b.Fatal(err)
	}

	// fields lists the plaintexts to encrypt, by name.
	fields := []struct {
		name      string
		plaintext string
	}{
		{name: "short", plaintext: "Backup finished"},
		{name: "1024", plaintext: strings.Repeat("x", 1024)},
	}

	for _, field := range fields {
		b.Run(field.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				if _, err := encryptField(field.plaintext, key); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
