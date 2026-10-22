package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Images and links the embed tests use.
const (
	// logoURL is the Shoutrrr logo.
	logoURL = "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-180px.png"
	// logotypeURL is the Shoutrrr logotype.
	logotypeURL = "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-logotype.png"
	// repositoryURL is the Shoutrrr repository.
	repositoryURL = "https://github.com/nicholas-fedor/shoutrrr"
)

// TestDiscordSendItems verifies that Discord accepts rich items as embeds with
// a level color, a timestamp, custom fields, images, author information,
// several embeds, file attachments, and all of these combined.
//
//nolint:paralleltest // Sends to a live webhook one message at a time.
func TestDiscordSendItems(t *testing.T) {
	// sends lists the rich items to send, by name.
	sends := []struct {
		name  string
		items []types.MessageItem
	}{
		{
			name:  "embed color (error level)",
			items: []types.MessageItem{{Text: "E2E Test: Error level embed (red color)", Level: types.Error}},
		},
		{
			name:  "embed timestamp",
			items: []types.MessageItem{{Text: "E2E Test: Embed with timestamp", Timestamp: time.Now()}},
		},
		{
			name: "embed with custom fields",
			items: []types.MessageItem{{
				Text: "E2E Test: Embed with custom fields",
				Fields: []types.Field{
					{Key: "Status", Value: "Testing"},
					{Key: "Environment", Value: "E2E"},
					{Key: "Feature", Value: "Custom Fields"},
				},
			}},
		},
		{
			name: "embed with images and thumbnails",
			items: []types.MessageItem{{
				Text: "E2E Test: Embed with image and thumbnail",
				Fields: []types.Field{
					{Key: "embed_image_url", Value: logoURL},
					{Key: "embed_thumbnail_url", Value: logotypeURL},
				},
			}},
		},
		{
			name: "embed with author information",
			items: []types.MessageItem{{
				Text: "E2E Test: Embed with author information",
				Fields: []types.Field{
					{Key: "embed_author_name", Value: "Shoutrrr E2E Test"},
					{Key: "embed_author_url", Value: repositoryURL},
					{Key: "embed_author_icon_url", Value: logoURL},
				},
			}},
		},
		{
			name: "multiple embeds",
			items: []types.MessageItem{
				{
					Text:   "E2E Test: First embed in multiple embeds",
					Level:  types.Info,
					Fields: []types.Field{{Key: "embed_author_name", Value: "First Embed"}},
				},
				{
					Text:   "E2E Test: Second embed in multiple embeds",
					Level:  types.Warning,
					Fields: []types.Field{{Key: "embed_author_name", Value: "Second Embed"}},
				},
			},
		},
		{
			name: "file attachment",
			items: []types.MessageItem{{
				Text: "E2E Test: File attachment",
				File: &types.File{Name: "e2e_test.txt", Data: []byte("E2E test file content for file attachment feature")},
			}},
		},
		{
			name: "multiple file attachments",
			items: []types.MessageItem{
				{
					Text: "E2E Test: Multiple file attachments - File 1",
					File: &types.File{Name: "test1.txt", Data: []byte("Content of first test file")},
				},
				{
					Text: "E2E Test: Multiple file attachments - File 2",
					File: &types.File{Name: "test2.txt", Data: []byte("Content of second test file")},
				},
			},
		},
		{
			name: "complex combination (embed with all features)",
			items: []types.MessageItem{{
				Text:      "E2E Test: Complex combination - embed with all features",
				Level:     types.Info,
				Timestamp: time.Now(),
				Fields: []types.Field{
					{Key: "embed_author_name", Value: "Shoutrrr E2E Test"},
					{Key: "embed_author_url", Value: repositoryURL},
					{Key: "embed_author_icon_url", Value: logoURL},
					{Key: "embed_image_url", Value: logotypeURL},
					{Key: "embed_thumbnail_url", Value: logoURL},
					{Key: "Status", Value: "Complex Test"},
					{Key: "Features", Value: "Author, Images, Fields, Timestamp"},
					{Key: "Level", Value: "Info"},
				},
			}},
		},
	}

	for _, tt := range sends {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, newService(t, nil).SendItemsContext(t.Context(), tt.items, nil))
		})
	}
}
