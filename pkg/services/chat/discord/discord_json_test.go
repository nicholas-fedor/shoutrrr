package discord

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// testLevelColors are the level colors the payload tests use.
var testLevelColors = [types.MessageLevelCount]uint{
	types.Unknown: 0,
	types.Debug:   0x0000ff,
	types.Info:    0x00ff00,
	types.Warning: 0xffff00,
	types.Error:   0xff0000,
}

// TestHasEmbedFields verifies that any field, embed or regular, makes an item
// need an embed.
func TestHasEmbedFields(t *testing.T) {
	t.Parallel()

	// tests lists fields with whether they need an embed.
	tests := []struct {
		name   string
		fields []types.Field
		want   bool
	}{
		{name: "no fields", fields: []types.Field{}, want: false},
		{name: "regular fields", fields: []types.Field{{Key: "regular_field", Value: "value"}, {Key: "another_field", Value: "another_value"}}, want: true},
		{name: "author name", fields: []types.Field{{Key: "regular_field", Value: "value"}, {Key: "embed_author_name", Value: "Author Name"}}, want: true},
		{name: "author URL", fields: []types.Field{{Key: "embed_author_url", Value: "https://example.com"}}, want: true},
		{name: "author icon", fields: []types.Field{{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"}}, want: true},
		{name: "image", fields: []types.Field{{Key: "embed_image_url", Value: "https://example.com/image.png"}}, want: true},
		{name: "thumbnail", fields: []types.Field{{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"}}, want: true},
		{
			name:   "several embed fields",
			fields: []types.Field{{Key: "embed_author_name", Value: "Author"}, {Key: "embed_image_url", Value: "https://example.com/image.png"}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, hasEmbedFields(tt.fields))
		})
	}
}

// TestProcessEmbedFields verifies that embed fields become the author, image,
// and thumbnail, the last value of a repeated key wins, and other fields become
// embed fields in order.
func TestProcessEmbedFields(t *testing.T) {
	t.Parallel()

	// tests lists fields with the embed parts they should produce.
	tests := []struct {
		name          string
		fields        []types.Field
		wantAuthor    *embedAuthor
		wantImage     *embedImage
		wantThumbnail *embedThumbnail
		wantFields    []embedField
	}{
		{name: "no fields", fields: []types.Field{}},
		{
			name:       "regular fields",
			fields:     []types.Field{{Key: "field1", Value: "value1"}, {Key: "field2", Value: "value2"}},
			wantFields: []embedField{{Name: "field1", Value: "value1"}, {Name: "field2", Value: "value2"}},
		},
		{name: "author name", fields: []types.Field{{Key: "embed_author_name", Value: "Test Author"}}, wantAuthor: &embedAuthor{Name: "Test Author"}},
		{name: "author URL", fields: []types.Field{{Key: "embed_author_url", Value: "https://example.com"}}, wantAuthor: &embedAuthor{URL: "https://example.com"}},
		{
			name:       "author icon",
			fields:     []types.Field{{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"}},
			wantAuthor: &embedAuthor{IconURL: "https://example.com/icon.png"},
		},
		{
			name: "full author",
			fields: []types.Field{
				{Key: "embed_author_name", Value: "Test Author"},
				{Key: "embed_author_url", Value: "https://example.com"},
				{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
			},
			wantAuthor: &embedAuthor{Name: "Test Author", URL: "https://example.com", IconURL: "https://example.com/icon.png"},
		},
		{name: "image", fields: []types.Field{{Key: "embed_image_url", Value: "https://example.com/image.png"}}, wantImage: &embedImage{URL: "https://example.com/image.png"}},
		{
			name:          "thumbnail",
			fields:        []types.Field{{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"}},
			wantThumbnail: &embedThumbnail{URL: "https://example.com/thumb.png"},
		},
		{
			name: "embed and regular fields",
			fields: []types.Field{
				{Key: "embed_author_name", Value: "Test Author"},
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
				{Key: "regular_field", Value: "regular_value"},
				{Key: "another_field", Value: "another_value"},
			},
			wantAuthor: &embedAuthor{Name: "Test Author"},
			wantImage:  &embedImage{URL: "https://example.com/image.png"},
			wantFields: []embedField{{Name: "regular_field", Value: "regular_value"}, {Name: "another_field", Value: "another_value"}},
		},
		{
			name:       "repeated key",
			fields:     []types.Field{{Key: "embed_author_name", Value: "First Author"}, {Key: "embed_author_name", Value: "Second Author"}},
			wantAuthor: &embedAuthor{Name: "Second Author"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			author, image, thumbnail, fields := processEmbedFields(tt.fields)

			assert.Equal(t, tt.wantAuthor, author)
			assert.Equal(t, tt.wantImage, image)
			assert.Equal(t, tt.wantThumbnail, thumbnail)

			if len(tt.wantFields) == 0 {
				assert.Empty(t, fields)
			} else {
				assert.Equal(t, tt.wantFields, fields)
			}
		})
	}
}

// TestCreatePayloadFromItemsPlainContent verifies that a single item without a
// title, level, timestamp, fields, or file becomes plain content.
func TestCreatePayloadFromItemsPlainContent(t *testing.T) {
	t.Parallel()

	// tests lists items that should stay plain content.
	tests := []struct {
		name string
		item types.MessageItem
	}{
		{name: "plain text", item: types.MessageItem{Text: "Simple message"}},
		{name: "zero timestamp", item: types.MessageItem{Text: "Message with zero timestamp", Timestamp: time.Time{}}},
		{name: "unknown level", item: types.MessageItem{Text: "Message with unknown level", Level: types.Unknown}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := CreatePayloadFromItems([]types.MessageItem{tt.item}, "", testLevelColors)
			require.NoError(t, err)

			assert.Equal(t, tt.item.Text, payload.Content)
			assert.Empty(t, payload.Embeds)
			assert.Empty(t, payload.Attachments)
		})
	}
}

// TestCreatePayloadFromItemsEmbeds verifies that a title, level, timestamp,
// embed fields, or file turns items into embeds with the matching parts.
func TestCreatePayloadFromItemsEmbeds(t *testing.T) {
	t.Parallel()

	t.Run("title", func(t *testing.T) {
		t.Parallel()

		payload, err := CreatePayloadFromItems([]types.MessageItem{{Text: "Message with title"}}, "Test Title", testLevelColors)
		require.NoError(t, err)

		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 1)
		assert.Equal(t, "Test Title", payload.Embeds[0].Title)
		assert.Equal(t, "Message with title", payload.Embeds[0].Content)
	})

	t.Run("level", func(t *testing.T) {
		t.Parallel()

		payload, err := CreatePayloadFromItems([]types.MessageItem{{Text: "Info message", Level: types.Info}}, "", testLevelColors)
		require.NoError(t, err)

		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 1)
		assert.Equal(t, uint(0x00ff00), payload.Embeds[0].Color)
		require.NotNil(t, payload.Embeds[0].Footer)
		assert.Equal(t, "Info", payload.Embeds[0].Footer.Text)
	})

	t.Run("level out of range", func(t *testing.T) {
		t.Parallel()

		payload, err := CreatePayloadFromItems([]types.MessageItem{{Text: "Message with invalid level", Level: types.MessageLevel(10)}}, "", testLevelColors)
		require.NoError(t, err)

		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 1)
		assert.Zero(t, payload.Embeds[0].Color)
	})

	t.Run("timestamp", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

		payload, err := CreatePayloadFromItems([]types.MessageItem{{Text: "Message with timestamp", Timestamp: timestamp}}, "", testLevelColors)
		require.NoError(t, err)

		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 1)
		assert.Equal(t, "2023-01-01T12:00:00Z", payload.Embeds[0].Timestamp)
	})

	t.Run("embed fields", func(t *testing.T) {
		t.Parallel()

		items := []types.MessageItem{{
			Text: "Message with complex embed",
			Fields: []types.Field{
				{Key: "embed_author_name", Value: "Author Name"},
				{Key: "embed_author_url", Value: "https://example.com"},
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
				{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
				{Key: "regular_field", Value: "regular_value"},
			},
		}}

		payload, err := CreatePayloadFromItems(items, "", testLevelColors)
		require.NoError(t, err)

		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 1)

		embed := payload.Embeds[0]
		assert.Equal(t, &embedAuthor{Name: "Author Name", URL: "https://example.com"}, embed.Author)
		assert.Equal(t, &embedImage{URL: "https://example.com/image.png"}, embed.Image)
		assert.Equal(t, &embedThumbnail{URL: "https://example.com/thumb.png"}, embed.Thumbnail)
		assert.Equal(t, []embedField{{Name: "regular_field", Value: "regular_value"}}, embed.Fields)
	})

	t.Run("several items", func(t *testing.T) {
		t.Parallel()

		items := []types.MessageItem{{Text: "First message", Level: types.Info}, {Text: "Second message", Level: types.Warning}}

		payload, err := CreatePayloadFromItems(items, "Multi Message", testLevelColors)
		require.NoError(t, err)

		assert.Empty(t, payload.Content)
		require.Len(t, payload.Embeds, 2)
		assert.Equal(t, "Multi Message", payload.Embeds[0].Title)
		assert.Equal(t, uint(0x00ff00), payload.Embeds[0].Color)
		assert.Equal(t, uint(0xffff00), payload.Embeds[1].Color)
	})
}

// TestCreatePayloadFromItemsAttachments verifies that each file becomes an
// attachment numbered in order.
func TestCreatePayloadFromItemsAttachments(t *testing.T) {
	t.Parallel()

	items := []types.MessageItem{
		{Text: "First file", File: &types.File{Name: "file1.txt", Data: []byte("content1")}},
		{Text: "Second file", File: &types.File{Name: "file2.txt", Data: []byte("content2")}},
	}

	payload, err := CreatePayloadFromItems(items, "", testLevelColors)
	require.NoError(t, err)

	assert.Empty(t, payload.Content)
	assert.Len(t, payload.Embeds, 2)
	assert.Equal(t, []attachment{{ID: 0, Filename: "file1.txt"}, {ID: 1, Filename: "file2.txt"}}, payload.Attachments)
}

// TestCreatePayloadFromItemsTotalLength verifies that embeds past Discord's
// combined limit of 6000 characters, which counts the author names as well as
// the descriptions, are shortened to the characters left, so a payload built
// directly from long items stays within the limit.
func TestCreatePayloadFromItemsTotalLength(t *testing.T) {
	t.Parallel()

	author := []types.Field{{Key: "embed_author_name", Value: strings.Repeat("n", 256)}}

	// tests lists items with the description the last embed should keep.
	tests := []struct {
		name     string
		items    []types.MessageItem
		wantLast string
	}{
		{
			name:     "descriptions",
			items:    []types.MessageItem{{Text: strings.Repeat("a", 4000)}, {Text: strings.Repeat("b", 4000)}},
			wantLast: strings.Repeat("b", TotalChunkSize-4000-len(" [...]")) + " [...]",
		},
		{
			name:     "author name",
			items:    []types.MessageItem{{Text: strings.Repeat("a", 3000), Fields: author}, {Text: strings.Repeat("b", 3000)}},
			wantLast: strings.Repeat("b", TotalChunkSize-3000-256-len(" [...]")) + " [...]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := CreatePayloadFromItems(tt.items, "", testLevelColors)
			require.NoError(t, err)

			require.Len(t, payload.Embeds, len(tt.items))
			assert.Equal(t, tt.items[0].Text, payload.Embeds[0].Content)
			assert.Equal(t, tt.wantLast, payload.Embeds[len(tt.items)-1].Content)
		})
	}
}

// TestCreatePayloadFromItemsOverBudget verifies that an embed with no
// characters left in Discord's combined limit of 6000 loses its description,
// author, and footer, which Discord does not accept empty, so the payload
// stays within the limit.
func TestCreatePayloadFromItemsOverBudget(t *testing.T) {
	t.Parallel()

	items := []types.MessageItem{
		{Text: strings.Repeat("a", 4000)},
		{Text: strings.Repeat("b", 2000)},
		{Text: "late", Level: types.Error, Fields: []types.Field{{Key: "embed_author_name", Value: strings.Repeat("n", 256)}}},
	}

	payload, err := CreatePayloadFromItems(items, "", testLevelColors)
	require.NoError(t, err)

	require.Len(t, payload.Embeds, 3)
	assert.Empty(t, payload.Embeds[2].Content)
	assert.Nil(t, payload.Embeds[2].Author)
	assert.Nil(t, payload.Embeds[2].Footer)

	total := 0
	for i := range payload.Embeds {
		total += embedLength(&payload.Embeds[i])
	}

	assert.Equal(t, TotalChunkSize, total)
}

// TestFitEmbed verifies that an embed over its budget gives up characters in
// order, from its fields, description, author name, and footer, and keeps an
// author or footer it shortens only while it still has text.
func TestFitEmbed(t *testing.T) {
	t.Parallel()

	// newTestEmbed returns an embed with a 10-character description, author
	// name, and footer.
	newTestEmbed := func() embedItem {
		return embedItem{
			Content: strings.Repeat("d", 10),
			Author:  &embedAuthor{Name: strings.Repeat("n", 10)},
			Footer:  &embedFooter{Text: strings.Repeat("f", 10)},
		}
	}

	// tests lists budgets with the embed each should leave.
	tests := []struct {
		name   string
		budget int
		want   embedItem
	}{
		{name: "fits", budget: 30, want: newTestEmbed()},
		{
			name:   "author shortened",
			budget: 15,
			want:   embedItem{Author: &embedAuthor{Name: strings.Repeat("n", 5)}, Footer: &embedFooter{Text: strings.Repeat("f", 10)}},
		},
		{name: "footer shortened", budget: 4, want: embedItem{Footer: &embedFooter{Text: strings.Repeat("f", 4)}}},
		{name: "negative budget", budget: -5, want: embedItem{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			embed := newTestEmbed()
			fitEmbed(&embed, tt.budget)

			assert.Equal(t, tt.want, embed)
		})
	}
}

// TestCreatePayloadFromItemsLimits verifies that no items or a single item
// without text are rejected and that embeds are capped at MaxEmbeds.
func TestCreatePayloadFromItemsLimits(t *testing.T) {
	t.Parallel()

	t.Run("no items", func(t *testing.T) {
		t.Parallel()

		_, err := CreatePayloadFromItems([]types.MessageItem{}, "", testLevelColors)
		require.ErrorIs(t, err, ErrEmptyMessage)
	})

	t.Run("single item without text", func(t *testing.T) {
		t.Parallel()

		_, err := CreatePayloadFromItems([]types.MessageItem{{Text: ""}}, "", testLevelColors)
		require.ErrorIs(t, err, ErrEmptyMessage)
	})

	t.Run("more items than embeds allow", func(t *testing.T) {
		t.Parallel()

		items := make([]types.MessageItem, MaxEmbeds+5)
		for i := range items {
			items[i] = types.MessageItem{Text: "Message " + strconv.Itoa(i)}
		}

		payload, err := CreatePayloadFromItems(items, "", testLevelColors)
		require.NoError(t, err)
		assert.Len(t, payload.Embeds, MaxEmbeds)
	})
}
