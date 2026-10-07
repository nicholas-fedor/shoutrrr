package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestPartitionMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		limits        types.MessageLimit
		distance      int
		wantItems     int
		wantOmitted   int
		wantFirstText string
	}{
		{
			name:  "partitions empty string",
			input: "",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     3,
			},
			distance:      10,
			wantItems:     0,
			wantOmitted:   0,
			wantFirstText: "",
		},
		{
			name:  "partitions short message without splitting",
			input: "Hello World",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     3,
			},
			distance:      10,
			wantItems:     1,
			wantOmitted:   0,
			wantFirstText: "Hello World",
		},
		{
			name:  "partitions message at whitespace",
			input: "Hello World this is a test message",
			limits: types.MessageLimit{
				ChunkSize:      15,
				TotalChunkSize: 100,
				ChunkCount:     5,
			},
			distance:      5,
			wantItems:     3,
			wantOmitted:   0,
			wantFirstText: "Hello World",
		},
		{
			name:  "handles message without whitespace",
			input: "ABCDEFGHIJ",
			limits: types.MessageLimit{
				ChunkSize:      5,
				TotalChunkSize: 100,
				ChunkCount:     5,
			},
			distance:      3,
			wantItems:     2,
			wantOmitted:   0,
			wantFirstText: "ABCDE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotOmitted := PartitionMessage(tt.input, tt.limits, tt.distance)

			assert.Len(t, got, tt.wantItems, "Number of items mismatch")
			assert.Equal(t, tt.wantOmitted, gotOmitted, "Omitted count mismatch")

			if tt.wantItems > 0 && tt.wantFirstText != "" {
				assert.Equal(t, tt.wantFirstText, got[0].Text, "First item text mismatch")
			}
		})
	}
}

func TestEllipsis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		maxLength int
		want      string
	}{
		{
			name:      "returns short text unchanged",
			text:      "Hello",
			maxLength: 10,
			want:      "Hello",
		},
		{
			name:      "truncates long text with ellipsis",
			text:      "Hello World",
			maxLength: 10,
			want:      "Hell [...]",
		},
		{
			name:      "handles exact length",
			text:      "HelloWorld",
			maxLength: 10,
			want:      "HelloWorld",
		},
		{
			name:      "handles empty string",
			text:      "",
			maxLength: 10,
			want:      "",
		},
		{
			name:      "counts runes, not bytes",
			text:      "Hello世界",
			maxLength: 8,
			want:      "Hello世界",
		},
		{
			name:      "truncates multi-byte text on rune boundaries",
			text:      "世界世界世界世界世界",
			maxLength: 8,
			want:      "世界 [...]",
		},
		{
			name:      "truncates without ellipsis when the limit is shorter than it",
			text:      "Hello World",
			maxLength: 3,
			want:      "Hel",
		},
		{
			name:      "returns empty text for a zero limit",
			text:      "Hello",
			maxLength: 0,
			want:      "",
		},
		{
			name:      "returns text unchanged when len equals maxLength",
			text:      "Hello",
			maxLength: 5,
			want:      "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Ellipsis(tt.text, tt.maxLength)
			assert.Equal(t, tt.want, got, "Ellipsis(%q, %d) result mismatch", tt.text, tt.maxLength)
		})
	}
}

func TestMessageItemsFromLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		plain             string
		limits            types.MessageLimit
		wantBatches       int
		wantFirstBatchLen int
	}{
		{
			name:  "returns empty for empty input",
			plain: "",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     3,
			},
			wantBatches:       0,
			wantFirstBatchLen: 0,
		},
		{
			name:  "creates single batch for single line",
			plain: "Hello World",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     3,
			},
			wantBatches:       1,
			wantFirstBatchLen: 1,
		},
		{
			name:  "splits multiple lines into items",
			plain: "Line1\nLine2\nLine3",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     5,
			},
			wantBatches:       1,
			wantFirstBatchLen: 3,
		},
		{
			name:  "respects chunk count limit",
			plain: "Line1\nLine2\nLine3\nLine4",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     2,
			},
			wantBatches:       2,
			wantFirstBatchLen: 2,
		},
		{
			name:  "truncates long lines",
			plain: strings.Repeat("A", 200),
			limits: types.MessageLimit{
				ChunkSize:      50,
				TotalChunkSize: 300,
				ChunkCount:     5,
			},
			wantBatches:       1,
			wantFirstBatchLen: 1,
		},
		{
			name:  "skips empty lines",
			plain: "Line1\n\nLine2",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     5,
			},
			wantBatches:       1,
			wantFirstBatchLen: 2,
		},
		{
			name:  "handles unicode lines",
			plain: "Hello\n世界\nTest",
			limits: types.MessageLimit{
				ChunkSize:      100,
				TotalChunkSize: 300,
				ChunkCount:     5,
			},
			wantBatches:       1,
			wantFirstBatchLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MessageItemsFromLines(tt.plain, tt.limits)

			assert.Len(t, got, tt.wantBatches, "Number of batches mismatch")

			if tt.wantBatches > 0 && tt.wantFirstBatchLen > 0 {
				assert.Len(t, got[0], tt.wantFirstBatchLen, "First batch length mismatch")
			}
		})
	}
}

func TestMessageItemsFromLines_Truncation(t *testing.T) {
	t.Parallel()

	// Test that long lines are properly truncated with ellipsis
	t.Run("truncates long lines with ellipsis", func(t *testing.T) {
		t.Parallel()

		limits := types.MessageLimit{
			ChunkSize:      20,
			TotalChunkSize: 100,
			ChunkCount:     5,
		}

		longLine := strings.Repeat("A", 50)
		batches := MessageItemsFromLines(longLine, limits)

		assert.Len(t, batches, 1)
		assert.Len(t, batches[0], 1)

		// Should be truncated to chunk size with ellipsis
		assert.Len(t, batches[0][0].Text, 20)
		assert.True(t, strings.HasSuffix(batches[0][0].Text, " [...]"), "Truncated text should end with ellipsis")
	})
}

// TestMessageItemsFromLinesContent verifies the text and grouping of each batch,
// not just batch counts, so lines are never lost, duplicated or reordered when
// they span several batches.
func TestMessageItemsFromLinesContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		plain  string
		limits types.MessageLimit
		want   [][]string
	}{
		{
			name:   "splits by chunk count",
			plain:  "a\nb\nc\nd",
			limits: types.MessageLimit{ChunkSize: 100, TotalChunkSize: 300, ChunkCount: 2},
			want:   [][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			name:   "splits by total size",
			plain:  "aaaa\nbbbb\ncccc\ndddd",
			limits: types.MessageLimit{ChunkSize: 100, TotalChunkSize: 10, ChunkCount: 10},
			want:   [][]string{{"aaaa", "bbbb"}, {"cccc", "dddd"}},
		},
		{
			name:   "skips empty lines without emitting empty batches",
			plain:  "a\n\n\nb\n\nc",
			limits: types.MessageLimit{ChunkSize: 100, TotalChunkSize: 300, ChunkCount: 2},
			want:   [][]string{{"a", "b"}, {"c"}},
		},
		{
			name:   "keeps multi-byte lines intact",
			plain:  "世界\nこんにちは\n안녕",
			limits: types.MessageLimit{ChunkSize: 100, TotalChunkSize: 7, ChunkCount: 10},
			want:   [][]string{{"世界", "こんにちは"}, {"안녕"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MessageItemsFromLines(tt.plain, tt.limits)

			assert.Equal(t, tt.want, batchTexts(got))
		})
	}
}

// batchTexts returns the text of every item, grouped by batch.
//
// Parameters:
//   - batches: the message item batches.
//
// Returns:
//   - [][]string: the item texts in batch order.
func batchTexts(batches [][]types.MessageItem) [][]string {
	texts := make([][]string, 0, len(batches))

	for _, batch := range batches {
		batchText := make([]string, 0, len(batch))
		for _, item := range batch {
			batchText = append(batchText, item.Text)
		}

		texts = append(texts, batchText)
	}

	return texts
}
