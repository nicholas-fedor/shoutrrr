package util

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// FuzzMessageItemsFromLines checks that batching keeps every non-empty line exactly
// once and in order, keeps each batch within its limits, and never emits invalid
// UTF-8. The seed corpus covers ASCII, multi-byte text, empty lines and lines longer
// than the chunk size.
func FuzzMessageItemsFromLines(f *testing.F) {
	for _, seed := range []string{
		"",
		"a\nb\nc\nd",
		"a\n\n\nb\n\nc",
		"世界\nこんにちは\n안녕",
		strings.Repeat("x", 50) + "\nshort",
	} {
		f.Add(seed, 20, 30, 3)
	}

	f.Fuzz(func(t *testing.T, plain string, chunkSize, totalSize, chunkCount int) {
		if chunkSize < 1 || chunkSize > 200 || totalSize < chunkSize || totalSize > 2000 ||
			chunkCount < 1 || chunkCount > 20 || !utf8.ValidString(plain) {
			t.Skip()
		}

		limits := types.MessageLimit{ChunkSize: chunkSize, TotalChunkSize: totalSize, ChunkCount: chunkCount}

		var want []string

		for line := range strings.SplitSeq(plain, "\n") {
			if line != "" {
				want = append(want, Ellipsis(line, chunkSize))
			}
		}

		var got []string

		for _, batch := range MessageItemsFromLines(plain, limits) {
			if len(batch) == 0 || len(batch) > chunkCount {
				t.Fatalf("batch has %d items, want 1 to %d", len(batch), chunkCount)
			}

			total := 0

			for _, item := range batch {
				if !utf8.ValidString(item.Text) {
					t.Fatalf("item %q is not valid UTF-8", item.Text)
				}

				total += utf8.RuneCountInString(item.Text)
				got = append(got, item.Text)
			}

			if len(batch) > 1 && total > totalSize {
				t.Fatalf("batch holds %d runes, want at most %d", total, totalSize)
			}
		}

		if strings.Join(got, "\n") != strings.Join(want, "\n") || len(got) != len(want) {
			t.Fatalf("lines = %q, want %q", got, want)
		}
	})
}
