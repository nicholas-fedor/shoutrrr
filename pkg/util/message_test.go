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

//nolint:gosmopolitan // Unicode characters used intentionally for testing rune handling
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
			name:      "handles unicode - counts bytes not runes",
			text:      "Hello世界",
			maxLength: 8,
			want:      "He [...]",
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

//nolint:gosmopolitan // Unicode characters used intentionally for testing rune handling
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
