package slack

import (
	"strconv"
	"strings"
	"testing"
)

// BenchmarkCreateJSONPayload measures building a payload, which each send
// does once.
func BenchmarkCreateJSONPayload(b *testing.B) {
	config := &Config{Title: "Backup", Color: "good", Channel: "C0123456789"}

	for _, lineCount := range []int{1, 10, 100, 1000} {
		message := strings.TrimSuffix(strings.Repeat("Backup of volume finished\n", lineCount), "\n")

		b.Run(strconv.Itoa(lineCount), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				CreateJSONPayload(config, message)
			}
		})
	}
}

// BenchmarkToken measures the request forms of a token, which each API send
// and each webhook send builds once.
func BenchmarkToken(b *testing.B) {
	bot, err := ParseToken("xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N")
	if err != nil {
		b.Fatal(err)
	}

	hook, err := ParseToken("hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456")
	if err != nil {
		b.Fatal(err)
	}

	b.Run("authorization", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = bot.Authorization()
		}
	})

	b.Run("webhook_url", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = hook.WebhookURL()
		}
	})
}
