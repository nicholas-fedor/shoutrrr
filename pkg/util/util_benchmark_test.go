package util

import (
	"strings"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Message splitting limits matching Discord's, the service that splits messages.
const (
	// benchChunkSize is the largest chunk, in runes.
	benchChunkSize = 2000
	// benchTotalChunkSize is the largest total of all chunks, in runes.
	benchTotalChunkSize = 6000
	// benchChunkCount is the most chunks, including the one kept for metadata.
	benchChunkCount = 10
	// benchSearchDistance is how far back, in runes, a split looks for whitespace.
	benchSearchDistance = 100
)

// benchLimits are the message limits the splitting benchmarks use.
var benchLimits = types.MessageLimit{
	ChunkSize:      benchChunkSize,
	TotalChunkSize: benchTotalChunkSize,
	ChunkCount:     benchChunkCount,
}

// benchMessages lists messages by name: one that fits in a single chunk, one
// that needs several, one longer than the total limit, and one of multibyte
// runes.
var benchMessages = []struct{ name, text string }{
	{name: "short", text: "Backup finished on host-01 in 42 seconds."},
	{name: "multiple", text: strings.Repeat("lorem ipsum dolor sit amet ", 200)},
	{name: "truncated", text: strings.Repeat("lorem ipsum dolor sit amet ", 400)},
	{name: "multibyte", text: strings.Repeat("世界 こんにちは 안녕 ", 400)},
}

// BenchmarkPartitionMessage measures splitting a message into chunks at word
// boundaries.
func BenchmarkPartitionMessage(b *testing.B) {
	for _, message := range benchMessages {
		b.Run(message.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_, _ = PartitionMessage(message.text, benchLimits, benchSearchDistance)
			}
		})
	}
}

// BenchmarkMessageItemsFromLines measures grouping the lines of a message into
// batches of items.
func BenchmarkMessageItemsFromLines(b *testing.B) {
	for _, message := range benchMessages {
		lines := strings.ReplaceAll(message.text, " ", "\n")

		b.Run(message.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = MessageItemsFromLines(lines, benchLimits)
			}
		})
	}
}

// BenchmarkEllipsis measures shortening a message to a rune limit.
func BenchmarkEllipsis(b *testing.B) {
	for _, message := range benchMessages {
		b.Run(message.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = Ellipsis(message.text, benchChunkSize)
			}
		})
	}
}

// BenchmarkNonEmpty measures dropping the empty elements of a list setting, as
// each send of a service with one does.
//
// Most lists hold no empty element, so that case must not allocate.
func BenchmarkNonEmpty(b *testing.B) {
	// lists holds three chats without an empty element, and the same chats
	// with the one a trailing comma leaves.
	lists := []struct {
		name   string
		values []string
	}{
		{name: "none empty", values: []string{"@alerts", "123456789", "@ops"}},
		{name: "trailing empty", values: []string{"@alerts", "123456789", "@ops", ""}},
	}

	for _, list := range lists {
		b.Run(list.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = NonEmpty(list.values)
			}
		})
	}
}

// BenchmarkValidateHostName measures checking a hostname that a config keeps
// apart from its port, as each send of such a service does.
//
// A hostname and an IPv4 address are the usual hosts, so they must not
// allocate.
func BenchmarkValidateHostName(b *testing.B) {
	// hosts lists a hostname, an IPv4 address, and an IPv6 address.
	hosts := []struct {
		name string
		host string
	}{
		{name: "hostname", host: "rocketchat.example.com"},
		{name: "ipv4", host: "192.0.2.1"},
		{name: "ipv6", host: "2001:db8::1"},
	}

	for _, tc := range hosts {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				if err := ValidateHostName(tc.host); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
