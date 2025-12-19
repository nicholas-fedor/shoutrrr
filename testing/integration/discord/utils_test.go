package discord_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"github.com/jarcoal/httpmock"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var setupOnce sync.Once

// SetupTestEnvironmentOnce sets up the test environment once.
func SetupTestEnvironmentOnce() {
	setupOnce.Do(func() {
		// Any one-time setup can go here
	})
}

// CreateDummyConfig creates a dummy Discord config for testing.
func CreateDummyConfig() discord.Config {
	return discord.Config{
		WebhookID:  "123456789012345678",
		Token:      "test-token-abcdefghijklmnopqrstuvwxyz123456",
		Title:      "",
		Username:   "",
		Avatar:     "",
		Color:      0x50D9ff,
		ColorError: 0xd60510,
		ColorWarn:  0xffc441,
		ColorInfo:  0x2488ff,
		ColorDebug: 0x7b00ab,
		SplitLines: false,
		JSON:       false,
		ThreadID:   "",
	}
}

// SetupMockResponder sets up httpmock to respond with the given status code for the config.
func SetupMockResponder(config *discord.Config, statusCode int) {
	apiURL := discord.CreateAPIURLFromConfig(config)
	if apiURL == "" {
		return
	}

	httpmock.RegisterResponder("POST", apiURL, httpmock.NewStringResponder(statusCode, ""))
}

// CreateTestService creates a test Discord service with the given config.
func CreateTestService(config discord.Config) *discord.Service {
	service := &discord.Service{}

	// Create a URL from the config to initialize the service
	configURL := config.GetURL()
	service.Initialize(configURL, nil)

	return service
}

// CreateMessageItem creates a message item with the given text.
func CreateMessageItem(text string) []types.MessageItem {
	return []types.MessageItem{
		{
			Text:  text,
			Level: types.Unknown,
		},
	}
}

// CreateMessageItemWithLevel creates a message item with the given text and level.
func CreateMessageItemWithLevel(text string, level types.MessageLevel) []types.MessageItem {
	return []types.MessageItem{
		{
			Text:  text,
			Level: level,
		},
	}
}

// CreateMessageItemWithFile creates a message item with text and a file attachment.
func CreateMessageItemWithFile(text string, filename string, data []byte) []types.MessageItem {
	return []types.MessageItem{
		{
			Text:  text,
			Level: types.Unknown,
			File: &types.File{
				Name: filename,
				Data: data,
			},
		},
	}
}

// CreateMessageItemWithFields creates a message item with text and fields.
func CreateMessageItemWithFields(text string, fields []types.Field) []types.MessageItem {
	return []types.MessageItem{
		{
			Text:   text,
			Level:  types.Unknown,
			Fields: fields,
		},
	}
}

// PayloadValidator is a function that validates a webhook payload.
type PayloadValidator func(payload discord.WebhookPayload) error

// MultipartValidator is a function that validates multipart form data.
type MultipartValidator func(payload discord.WebhookPayload, files []types.File, contentType string, body []byte) error

// SetupMockResponderWithPayloadValidation sets up httpmock to validate JSON payloads.
func SetupMockResponderWithPayloadValidation(
	config *discord.Config,
	statusCode int,
	validator PayloadValidator,
) {
	apiURL := discord.CreateAPIURLFromConfig(config)
	if apiURL == "" {
		return
	}

	httpmock.RegisterResponder("POST", apiURL, func(req *http.Request) (*http.Response, error) {
		// Read the request body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return httpmock.NewStringResponse(
				500,
				"Failed to read request body",
			), err
		}

		// Parse the JSON payload
		var payload discord.WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return httpmock.NewStringResponse(400, "Invalid JSON payload"), nil //nolint:nilerr
		}

		// Validate the payload
		if validator != nil {
			if err := validator(payload); err != nil {
				// Validation failed - return error status so test fails
				return httpmock.NewStringResponse(
					400,
					"Validation failed: "+err.Error(),
				), err
			}
		}

		return httpmock.NewStringResponse(statusCode, ""), nil
	})
}

// SetupMockResponderWithMultipartValidation sets up httpmock to validate multipart payloads.
func SetupMockResponderWithMultipartValidation(
	config *discord.Config,
	statusCode int,
	validator MultipartValidator,
) {
	apiURL := discord.CreateAPIURLFromConfig(config)
	if apiURL == "" {
		return
	}

	httpmock.RegisterResponder("POST", apiURL, func(req *http.Request) (*http.Response, error) {
		// Read the request body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return httpmock.NewStringResponse(
				500,
				"Failed to read request body",
			), err
		}

		contentType := req.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			return httpmock.NewStringResponse(400, "Expected multipart/form-data"), nil
		}

		// Parse multipart form
		boundary := strings.TrimPrefix(contentType, "multipart/form-data; boundary=")
		reader := multipart.NewReader(bytes.NewReader(body), boundary)

		var (
			payload discord.WebhookPayload
			files   []types.File
		)

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}

			if err != nil {
				return httpmock.NewStringResponse(
					400,
					"Failed to parse multipart",
				), err
			}

			filename := part.FileName()
			if filename != "" {
				// This is a file part
				fileData, err := io.ReadAll(part)
				if err != nil {
					return httpmock.NewStringResponse(
						400,
						"Failed to read file data",
					), err
				}

				files = append(files, types.File{
					Name: filename,
					Data: fileData,
				})
			} else {
				// This should be the payload_json field
				fieldName := part.FormName()
				if fieldName == "payload_json" {
					payloadData, err := io.ReadAll(part)
					if err != nil {
						return httpmock.NewStringResponse(400, "Failed to read payload_json"), nil //nolint:nilerr
					}

					if err := json.Unmarshal(payloadData, &payload); err != nil {
						return httpmock.NewStringResponse(400, "Invalid JSON in payload_json"), nil //nolint:nilerr
					}
				}
			}

			part.Close()
		}

		// Validate the multipart data
		if validator != nil {
			if err := validator(payload, files, contentType, body); err != nil {
				return httpmock.NewStringResponse(400, err.Error()), nil
			}
		}

		return httpmock.NewStringResponse(statusCode, ""), nil
	})
}

// ValidatePlainTextPayload validates that a payload contains plain text content.
func ValidatePlainTextPayload(expectedContent string) PayloadValidator {
	return func(payload discord.WebhookPayload) error {
		if payload.Content != expectedContent {
			return fmt.Errorf("expected content %q, got %q", expectedContent, payload.Content)
		}

		if len(payload.Embeds) > 0 {
			return fmt.Errorf("expected no embeds for plain text, got %d", len(payload.Embeds))
		}

		return nil
	}
}

// ValidateEmbedPayload validates that a payload contains embeds with expected properties.
func ValidateEmbedPayload(expectedText string, expectedColor uint) PayloadValidator {
	return func(payload discord.WebhookPayload) error {
		if len(payload.Embeds) == 0 {
			return errors.New("expected at least one embed, got none")
		}

		embed := payload.Embeds[0]
		if embed.Content != expectedText {
			return fmt.Errorf("expected embed description %q, got %q", expectedText, embed.Content)
		}

		if embed.Color != expectedColor {
			return fmt.Errorf("expected embed color %d, got %d", expectedColor, embed.Color)
		}

		return nil
	}
}

// ValidateUsername validates that a payload contains the expected username.
func ValidateUsername(expectedUsername string) PayloadValidator {
	return func(payload discord.WebhookPayload) error {
		if payload.Username != expectedUsername {
			return fmt.Errorf("expected username %q, got %q", expectedUsername, payload.Username)
		}

		return nil
	}
}

// ValidateAvatarURL validates that a payload contains the expected avatar URL.
func ValidateAvatarURL(expectedAvatarURL string) PayloadValidator {
	return func(payload discord.WebhookPayload) error {
		if payload.AvatarURL != expectedAvatarURL {
			return fmt.Errorf(
				"expected avatar URL %q, got %q",
				expectedAvatarURL,
				payload.AvatarURL,
			)
		}

		return nil
	}
}

// ValidateMultipartFiles validates that multipart data contains expected files.
func ValidateMultipartFiles(expectedFiles []types.File) MultipartValidator {
	return func(_ discord.WebhookPayload, files []types.File, contentType string, body []byte) error { //nolint:revive
		if len(files) != len(expectedFiles) {
			return fmt.Errorf("expected %d files, got %d", len(expectedFiles), len(files))
		}

		for i, expectedFile := range expectedFiles {
			if i >= len(files) {
				return fmt.Errorf("missing file at index %d", i)
			}

			actualFile := files[i]
			if actualFile.Name != expectedFile.Name {
				return fmt.Errorf(
					"expected filename %q, got %q",
					expectedFile.Name,
					actualFile.Name,
				)
			}

			if !bytes.Equal(actualFile.Data, expectedFile.Data) {
				return fmt.Errorf("file data mismatch for %s", actualFile.Name)
			}
		}

		return nil
	}
}

// ValidateDiscordWebhookAPISpec validates that the payload conforms to Discord webhook API specification.
func ValidateDiscordWebhookAPISpec() PayloadValidator {
	return func(payload discord.WebhookPayload) error {
		// Discord webhook API specification validation

		// Content field validation
		if payload.Content != "" && len(payload.Content) > 2000 {
			return fmt.Errorf(
				"content field exceeds 2000 character limit: got %d",
				len(payload.Content),
			)
		}

		// Username validation (optional field)
		if payload.Username != "" && len(payload.Username) > 80 {
			return fmt.Errorf(
				"username field exceeds 80 character limit: got %d",
				len(payload.Username),
			)
		}

		// Avatar URL validation (optional field)
		if payload.AvatarURL != "" && len(payload.AvatarURL) > 2048 {
			return fmt.Errorf(
				"avatar_url field exceeds 2048 character limit: got %d",
				len(payload.AvatarURL),
			)
		}

		// Embeds validation
		if len(payload.Embeds) > 10 {
			return fmt.Errorf("embeds array exceeds 10 item limit: got %d", len(payload.Embeds))
		}

		for i, embed := range payload.Embeds {
			if err := validateEmbedItem(embed, i); err != nil {
				return err
			}
		}

		// Attachments validation
		if len(payload.Attachments) > 10 {
			return fmt.Errorf(
				"attachments array exceeds 10 item limit: got %d",
				len(payload.Attachments),
			)
		}

		for i, attachment := range payload.Attachments {
			if attachment.ID != i {
				return fmt.Errorf(
					"attachment %d has incorrect ID: expected %d, got %d",
					i,
					i,
					attachment.ID,
				)
			}

			if attachment.Filename == "" {
				return fmt.Errorf("attachment %d has empty filename", i)
			}

			if len(attachment.Filename) > 260 {
				return fmt.Errorf(
					"attachment %d filename exceeds 260 character limit: got %d",
					i,
					len(attachment.Filename),
				)
			}
		}

		return nil
	}
}

// validateEmbedItem validates a single embed item against Discord API specification.
func validateEmbedItem(_ any, index int) error { //nolint:revive
	// This is a simplified validation - in a real implementation you'd need to reflect on the embedItem struct
	// For now, we'll do basic validation that can be expanded

	// Note: This is a placeholder for more comprehensive embed validation
	// In practice, you'd validate each field of the embedItem struct against Discord's limits
	return nil
}

// ValidateMultipartFormDataSpec validates multipart form data against Discord webhook API specification.
func ValidateMultipartFormDataSpec(expectedFiles []types.File) MultipartValidator {
	return func(payload discord.WebhookPayload, files []types.File, contentType string, _ []byte) error {
		// Validate Content-Type header
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			return fmt.Errorf(
				"expected Content-Type to start with 'multipart/form-data', got %q",
				contentType,
			)
		}

		// Validate boundary is present
		if !strings.Contains(contentType, "boundary=") {
			return fmt.Errorf("Content-Type missing boundary parameter: %q", contentType)
		}

		// Validate payload_json field exists and is valid JSON
		if payload.Content == "" && len(payload.Embeds) == 0 {
			return errors.New("payload_json must contain either content or embeds")
		}

		// Validate files
		if len(files) != len(expectedFiles) {
			return fmt.Errorf("expected %d files, got %d", len(expectedFiles), len(files))
		}

		for i, expectedFile := range expectedFiles {
			if i >= len(files) {
				return fmt.Errorf("missing file at index %d", i)
			}

			actualFile := files[i]
			if actualFile.Name != expectedFile.Name {
				return fmt.Errorf(
					"expected filename %q, got %q",
					expectedFile.Name,
					actualFile.Name,
				)
			}

			if !bytes.Equal(actualFile.Data, expectedFile.Data) {
				return fmt.Errorf("file data mismatch for %s", actualFile.Name)
			}
			// Discord limits file size to 8MB per file
			if len(actualFile.Data) > 8*1024*1024 {
				return fmt.Errorf(
					"file %s exceeds 8MB limit: got %d bytes",
					actualFile.Name,
					len(actualFile.Data),
				)
			}
		}

		return nil
	}
}

// ValidateAPIURLConstruction validates that API URLs are constructed correctly with thread parameters.
func ValidateAPIURLConstruction(expectedURL string) func(config *discord.Config) error {
	return func(config *discord.Config) error {
		actualURL := discord.CreateAPIURLFromConfig(config)
		if actualURL != expectedURL {
			return fmt.Errorf("expected API URL %q, got %q", expectedURL, actualURL)
		}

		return nil
	}
}

// ValidateComponentInteraction validates the interaction between payload creation and HTTP sending.
func ValidateComponentInteraction(
	expectedPayloadChecks ...PayloadValidator,
) func(payload discord.WebhookPayload) error {
	return func(payload discord.WebhookPayload) error {
		for _, check := range expectedPayloadChecks {
			if err := check(payload); err != nil {
				return fmt.Errorf("component interaction validation failed: %w", err)
			}
		}

		return nil
	}
}

// ValidateHTTPRequest validates the actual HTTP request being made.
type HTTPRequestValidator func(req *http.Request) error

// SetupMockResponderWithHTTPRequestValidation sets up httpmock to validate the HTTP request itself.
func SetupMockResponderWithHTTPRequestValidation(
	config *discord.Config,
	statusCode int,
	requestValidator HTTPRequestValidator,
	payloadValidator PayloadValidator,
) {
	apiURL := discord.CreateAPIURLFromConfig(config)
	if apiURL == "" {
		return
	}

	httpmock.RegisterResponder("POST", apiURL, func(req *http.Request) (*http.Response, error) {
		// Validate the HTTP request
		if requestValidator != nil {
			if err := requestValidator(req); err != nil {
				return httpmock.NewStringResponse(
					400,
					"HTTP request validation failed: "+err.Error(),
				), err
			}
		}

		// Read the request body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return httpmock.NewStringResponse(
				500,
				"Failed to read request body",
			), err
		}

		// Parse the JSON payload
		var payload discord.WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return httpmock.NewStringResponse(400, "Invalid JSON payload"), nil //nolint:nilerr
		}

		// Validate the payload
		if payloadValidator != nil {
			if err := payloadValidator(payload); err != nil {
				return httpmock.NewStringResponse(
					400,
					"Payload validation failed: "+err.Error(),
				), err
			}
		}

		return httpmock.NewStringResponse(statusCode, ""), nil
	})
}

// SetupMockResponderWithMultipartHTTPRequestValidation sets up httpmock to validate multipart HTTP requests.
func SetupMockResponderWithMultipartHTTPRequestValidation(
	config *discord.Config,
	statusCode int,
	requestValidator HTTPRequestValidator,
	multipartValidator MultipartValidator,
) {
	apiURL := discord.CreateAPIURLFromConfig(config)
	if apiURL == "" {
		return
	}

	httpmock.RegisterResponder("POST", apiURL, func(req *http.Request) (*http.Response, error) {
		// Validate the HTTP request
		if requestValidator != nil {
			if err := requestValidator(req); err != nil {
				return httpmock.NewStringResponse(
					400,
					"HTTP request validation failed: "+err.Error(),
				), err
			}
		}

		// Read the request body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return httpmock.NewStringResponse(
				500,
				"Failed to read request body",
			), err
		}

		contentType := req.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			return httpmock.NewStringResponse(400, "Expected multipart/form-data"), nil
		}

		// Parse multipart form
		boundary := strings.TrimPrefix(contentType, "multipart/form-data; boundary=")
		reader := multipart.NewReader(bytes.NewReader(body), boundary)

		var (
			payload discord.WebhookPayload
			files   []types.File
		)

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}

			if err != nil {
				return httpmock.NewStringResponse(
					400,
					"Failed to parse multipart",
				), err
			}

			filename := part.FileName()
			if filename != "" {
				// This is a file part
				fileData, err := io.ReadAll(part)
				if err != nil {
					return httpmock.NewStringResponse(
						400,
						"Failed to read file data",
					), err
				}

				files = append(files, types.File{
					Name: filename,
					Data: fileData,
				})
			} else {
				// This should be the payload_json field
				fieldName := part.FormName()
				if fieldName == "payload_json" {
					payloadData, err := io.ReadAll(part)
					if err != nil {
						return httpmock.NewStringResponse(400, "Failed to read payload_json"), nil //nolint:nilerr
					}

					if err := json.Unmarshal(payloadData, &payload); err != nil {
						return httpmock.NewStringResponse(400, "Invalid JSON in payload_json"), nil //nolint:nilerr
					}
				}
			}

			part.Close()
		}

		// Validate the multipart data
		if multipartValidator != nil {
			if err := multipartValidator(payload, files, contentType, body); err != nil {
				return httpmock.NewStringResponse(400, err.Error()), nil
			}
		}

		return httpmock.NewStringResponse(statusCode, ""), nil
	})
}

// ValidateHTTPMethod validates that the HTTP request uses the correct method.
func ValidateHTTPMethod(expectedMethod string) HTTPRequestValidator {
	return func(req *http.Request) error {
		if req.Method != expectedMethod {
			return fmt.Errorf("expected HTTP method %q, got %q", expectedMethod, req.Method)
		}

		return nil
	}
}

// ValidateHTTPHeaders validates HTTP request headers.
func ValidateHTTPHeaders(expectedHeaders map[string]string) HTTPRequestValidator {
	return func(req *http.Request) error {
		for key, expectedValue := range expectedHeaders {
			actualValue := req.Header.Get(key)
			if actualValue != expectedValue {
				return fmt.Errorf(
					"expected header %q to be %q, got %q",
					key,
					expectedValue,
					actualValue,
				)
			}
		}

		return nil
	}
}

// ValidateContentType validates the Content-Type header.
func ValidateContentType(expectedContentType string) HTTPRequestValidator {
	return func(req *http.Request) error {
		contentType := req.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, expectedContentType) {
			return fmt.Errorf(
				"expected Content-Type to start with %q, got %q",
				expectedContentType,
				contentType,
			)
		}

		return nil
	}
}

// ValidateRequestURL validates the request URL.
func ValidateRequestURL(expectedURL string) HTTPRequestValidator {
	return func(req *http.Request) error {
		if req.URL.String() != expectedURL {
			return fmt.Errorf("expected request URL %q, got %q", expectedURL, req.URL.String())
		}

		return nil
	}
}
