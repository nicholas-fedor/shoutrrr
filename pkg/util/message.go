package util

import (
	"strings"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ellipsis is the suffix appended to truncated strings.
const ellipsis = " [...]"

// PartitionMessage splits input into chunks that fit within the specified limits.
//
// The function attempts to split at word boundaries (spaces or newlines) to improve
// readability. It searches backwards up to distance runes from the chunk boundary
// to find a suitable split point.
//
// Parameters:
//   - input: The string to partition into chunks.
//   - limits: The message limits containing ChunkSize, TotalChunkSize, and ChunkCount.
//   - distance: The maximum number of runes to search backwards for a whitespace character.
//
// Returns:
//   - A slice of MessageItem containing the partitioned chunks.
//   - The number of runes that were omitted (exceeded limits).
func PartitionMessage(
	input string,
	limits types.MessageLimit,
	distance int,
) ([]types.MessageItem, int) {
	items := make([]types.MessageItem, 0, limits.ChunkCount-1)
	runes := []rune(input)
	chunkOffset := 0
	maxTotal := Min(len(runes), limits.TotalChunkSize)
	maxCount := limits.ChunkCount - 1

	if input == "" {
		// Return an empty slice for empty input.
		return items, 0
	}

	for range maxCount {
		// Default chunkEnd is the maximum chunk size from current offset.
		chunkEnd := chunkOffset + limits.ChunkSize
		// Default next chunk starts immediately after this one.
		nextChunkStart := chunkEnd

		if chunkEnd >= maxTotal {
			// The remaining content fits within limits; use all remaining runes.
			chunkEnd = maxTotal
			nextChunkStart = maxTotal
		} else {
			// Search backwards for a whitespace to split at a word boundary.
			for r := range distance {
				rp := chunkEnd - r
				if runes[rp] == '\n' || runes[rp] == ' ' {
					// Found a suitable split point at this whitespace.
					chunkEnd = rp
					// Skip the whitespace in the next chunk.
					nextChunkStart = chunkEnd + 1

					break
				}
			}
		}

		//nolint:exhaustruct_v5 // MessageItem only requires Text field for this use case
		items = append(items, types.MessageItem{
			Text: string(runes[chunkOffset:chunkEnd]),
		})

		chunkOffset = nextChunkStart
		if chunkOffset >= maxTotal {
			break
		}
	}

	return items, len(runes) - chunkOffset
}

// Ellipsis truncates text to maxLength runes, appending an ellipsis if truncated.
//
// Truncation happens on rune boundaries, so multi-byte characters are never split.
// When maxLength is too short to fit the ellipsis, the text is truncated without it.
//
// Parameters:
//   - text: The string to potentially truncate.
//   - maxLength: The maximum length in runes.
//
// Returns:
//   - The original text if it fits within maxLength, otherwise the truncated text,
//     with " [...]" appended when it fits.
func Ellipsis(text string, maxLength int) string {
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}

	if maxLength <= len(ellipsis) {
		return string(runes[:max(maxLength, 0)])
	}

	return string(runes[:maxLength-len(ellipsis)]) + ellipsis
}

// MessageItemsFromLines creates batches of MessageItem from newline-separated text.
//
// This function splits the input by newlines and creates batches that respect
// the chunk count and total chunk size limits, measured in runes. Empty lines are
// skipped, and individual lines that exceed ChunkSize are truncated with an ellipsis.
//
// Parameters:
//   - plain: The input text containing newline-separated lines.
//   - limits: The message limits for chunking (ChunkSize, TotalChunkSize, ChunkCount).
//
// Returns:
//   - A slice of message item batches, where each batch is a slice of MessageItem.
func MessageItemsFromLines(plain string, limits types.MessageLimit) [][]types.MessageItem {
	batches := make([][]types.MessageItem, 0)

	var (
		items       []types.MessageItem
		totalLength int
	)

	for line := range strings.SplitSeq(plain, "\n") {
		if line == "" {
			continue
		}

		line = Ellipsis(line, limits.ChunkSize)
		lineLength := utf8.RuneCountInString(line)

		if len(items) > 0 &&
			(len(items) == limits.ChunkCount || totalLength+lineLength > limits.TotalChunkSize) {
			// The current batch is full, so start a new one.
			batches = append(batches, items)
			items = nil
			totalLength = 0
		}

		//nolint:exhaustruct_v5 // MessageItem only requires Text field for this use case
		items = append(items, types.MessageItem{
			Text: line,
		})

		totalLength += lineLength
	}

	if len(items) > 0 {
		batches = append(batches, items)
	}

	return batches
}
