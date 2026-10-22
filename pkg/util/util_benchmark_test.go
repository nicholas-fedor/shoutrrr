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
