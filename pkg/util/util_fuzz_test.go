package util

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// FuzzPartitionMessage checks that partitioning keeps every chunk within its
// limits, never emits an empty chunk or invalid UTF-8, and only drops a single
// space or newline at each split, so the chunks, their separators, and the
// omitted runes rebuild the input. Limits that leave no room for a chunk yield
// no chunks and omit every rune. The seed corpus covers ASCII with and without
// whitespace, multi-byte text, newlines, input longer than the total size, a
// search distance longer than the chunk size, and limits of zero.
func FuzzPartitionMessage(f *testing.F) {
	for _, seed := range []string{
		"",
		"a",
		"lorem ipsum dolor sit amet consectetur adipiscing elit",
		strings.Repeat("x", 120),
		"世界 こんにちは 안녕 世界 こんにちは 안녕",
		"line one\nline two\nline three\n\n\nline four",
	} {
		f.Add(seed, 10, 50, 4, 5)
		f.Add(seed, 3, 50, 4, 20)
		f.Add(seed, 0, 0, 0, 0)
	}

	f.Fuzz(func(t *testing.T, input string, chunkSize, totalSize, chunkCount, distance int) {
		if chunkSize < -5 || chunkSize > 200 || totalSize < -5 || totalSize > 2000 ||
			chunkCount < -5 || chunkCount > 20 || distance < -5 || distance > 400 ||
			!utf8.ValidString(input) {
			t.Skip()
		}

		limits := types.MessageLimit{ChunkSize: chunkSize, TotalChunkSize: totalSize, ChunkCount: chunkCount}
		items, omitted := PartitionMessage(input, limits, distance)

		if chunkSize < 1 || totalSize < 1 || chunkCount < 1 {
			require.Empty(t, items, "limits without room yielded chunks")
			require.Equal(t, utf8.RuneCountInString(input), omitted, "limits without room must omit every rune")

			return
		}

		require.LessOrEqual(t, len(items), chunkCount, "too many chunks")

		runes := []rune(input)
		// ends holds the offsets where the last chunk can end, since a chunk
		// may follow a skipped space or newline or start with its own.
		ends := []int{0}
		total := 0

		for i, item := range items {
			chunk := []rune(item.Text)
			require.NotEmpty(t, chunk, "chunk %d is empty", i)
			require.LessOrEqual(t, len(chunk), chunkSize, "chunk %d %q is too long", i, item.Text)
			require.True(t, utf8.ValidString(item.Text), "chunk %d %q is not valid UTF-8", i, item.Text)

			total += len(chunk)

			var next []int

			for _, start := range splitStarts(runes, ends, i > 0) {
				if start+len(chunk) <= len(runes) && string(runes[start:start+len(chunk)]) == item.Text {
					next = append(next, start+len(chunk))
				}
			}

			require.NotEmpty(t, next, "chunk %d %q does not continue the input %q", i, item.Text, input)

			ends = next
		}

		require.LessOrEqual(t, total, totalSize, "the chunks hold too many runes")

		rests := make([]int, 0, len(ends))
		for _, start := range splitStarts(runes, ends, len(items) > 0) {
			rests = append(rests, len(runes)-start)
		}

		assert.Contains(t, rests, omitted, "omitted does not count the runes after the last chunk of %q", input)
	})
}

// FuzzEllipsis checks that shortened text stays within the limit as a rune prefix
// of the input, with the ellipsis only when the limit leaves room for it, and that
// text within the limit comes back unchanged. The seed corpus covers ASCII,
// multi-byte text, and limits around the ellipsis length, including negative ones.
func FuzzEllipsis(f *testing.F) {
	for _, seed := range []struct {
		text      string
		maxLength int
	}{
		{"", 0},
		{"hello", 10},
		{"hello world", 5},
		{"hello world", 6},
		{"hello world", 7},
		{"世界こんにちは안녕하세요", 8},
		{"truncate me", -1},
	} {
		f.Add(seed.text, seed.maxLength)
	}

	f.Fuzz(func(t *testing.T, text string, maxLength int) {
		if !utf8.ValidString(text) {
			t.Skip()
		}

		got := Ellipsis(text, maxLength)
		runes := []rune(text)

		if len(runes) <= maxLength {
			assert.Equal(t, text, got, "text within the limit must come back unchanged")

			return
		}

		require.LessOrEqual(t, utf8.RuneCountInString(got), max(maxLength, 0), "Ellipsis(%q, %d) = %q", text, maxLength, got)

		prefix := got

		if maxLength > len(ellipsis) {
			var found bool

			prefix, found = strings.CutSuffix(got, ellipsis)
			require.True(t, found, "Ellipsis(%q, %d) = %q, want the %q suffix", text, maxLength, got, ellipsis)
		}

		assert.True(t, strings.HasPrefix(text, prefix) && utf8.ValidString(prefix),
			"Ellipsis(%q, %d) = %q, want a rune prefix of the text", text, maxLength, got)
	})
}

// splitStarts returns the offsets where the next chunk or the omitted runes can
// start after chunks ending at ends: each end itself and, after a split, the
// offset past a space or newline at that end, without duplicates.
//
// Parameters:
//   - runes: the partitioned input.
//   - ends: the offsets where the previous chunk can end, in increasing order.
//   - split: whether a chunk precedes the offsets, so a separator may follow it.
//
// Returns:
//   - []int: the possible start offsets, in increasing order.
func splitStarts(runes []rune, ends []int, split bool) []int {
	starts := make([]int, 0, 2*len(ends))

	for _, end := range ends {
		starts = append(starts, end)

		if split && end < len(runes) && (runes[end] == ' ' || runes[end] == '\n') {
			starts = append(starts, end+1)
		}
	}

	// An end one past the previous end repeats its separator offset.
	return slices.Compact(starts)
}
