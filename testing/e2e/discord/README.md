# Discord End-to-End Tests

This directory contains end-to-end (e2e) tests for the Discord service in Shoutrrr.

## Overview

The end-to-end tests validate the complete Discord webhook functionality by making actual HTTP requests to Discord's API.

## Test Coverage

### Basic Message Functionality

- Plain text messages with various content types
- Special characters and Unicode handling
- Long message content (chunking behavior)

### Rich Embed Features

- Author information (name, URL, icon URL)
- Color customization and level-based coloring
- Custom fields with key-value pairs
- Image and thumbnail URLs
- Timestamps
- Multiple embeds in single messages

### File Attachments

- Single file attachments
- Multiple file attachments
- Different file types and sizes
- Special characters in filenames

### Thread Support

- Posting to existing threads using `thread_id`
- Thread permission and state handling

### Configuration Options

- Custom username and avatar overrides
- JSON mode for raw payload delivery
- Complex parameter combinations

### Advanced Scenarios

- Complex embeds with all features combined
- Edge cases and error conditions
- Rate limit handling with delays between tests

## Setup Requirements

### Discord Webhook Configuration

1. Create a Discord webhook in your server/channel:
   - Go to Server Settings → Integrations → Webhooks
   - Create a new webhook or use an existing one
   - Copy the webhook URL

2. (Optional) Create a thread for thread testing:
   - Post a message in your channel
   - Right-click the message → "Create Thread"
   - Copy the thread ID from the URL (the number after `/threads/`)

### Environment Variables

Create a `.env` file in this directory with the following variables:

```bash
# Required: Discord webhook URL
SHOUTRRR_DISCORD_URL=discord://YOUR_WEBHOOK_TOKEN@YOUR_WEBHOOK_ID

# Optional: Thread ID for thread testing
SHOUTRRR_DISCORD_THREAD_ID=YOUR_THREAD_ID
```

**Security Note**: Never commit real webhook URLs to version control. The `.env` file is already in `.gitignore`.

### Example .env Configuration

```bash
SHOUTRRR_DISCORD_URL=discord://<TOKEN>@<WEBHOOK_ID>
SHOUTRRR_DISCORD_THREAD_ID=<THREAD_ID>
```

## Running the Tests

### Prerequisites

- Go 1.25+
- Valid Discord webhook URL
- `.env` file with required environment variables

### Execute E2E Tests

Run all e2e tests:

```bash
go test ./testing/e2e/discord/ -v
```

Run specific test files:

```bash
# Test basic functionality
go test ./testing/e2e/discord/ -v -args -ginkgo.focus="basic"

# Test embed features
go test ./testing/e2e/discord/ -v -args -ginkgo.focus="embed"

# Test file attachments
go test ./testing/e2e/discord/ -v -args -ginkgo.focus="file"
```

### Test Behavior

- Tests will skip if required environment variables are missing
- Tests include delays between executions to respect Discord's rate limits
- Thread tests may skip if thread permissions prevent posting
- All tests send actual messages to your Discord channel - monitor your channel during test runs

### Expected Output

Successful test run will show messages appearing in your Discord channel. Each test includes an "E2E Test:" prefix to identify test messages.

## Test Structure

```bash
testing/e2e/discord/
├── .env                          # Environment variables (not committed)
├── suite_test.go                 # Test suite setup and configuration
├── basic_test.go                 # Basic text message functionality
├── complex_combination_test.go   # Complex embeds with all features
├── embed_author_test.go          # Embed author information
├── embed_colors_test.go          # Color customization
├── embed_fields_test.go          # Custom fields
├── embed_images_test.go          # Images and thumbnails
├── embed_timestamp_test.go       # Timestamp handling
├── file_attachment_test.go       # Single file attachments
├── json_mode_test.go             # JSON mode functionality
├── long_message_test.go          # Long message handling
├── multiple_embeds_test.go       # Multiple embeds
├── multiple_files_test.go        # Multiple file attachments
├── special_characters_test.go    # Special character handling
├── thread_test.go                # Thread functionality
└── username_avatar_test.go       # Username/avatar customization
```

### Test Organization

Each test file focuses on a specific feature category:

- **Basic Tests** (`basic_test.go`): Core message sending functionality
- **Embed Tests** (`embed_*_test.go`): Rich embed features and formatting
- **File Tests** (`file_*_test.go`): File attachment handling
- **Thread Tests** (`thread_test.go`): Posting messages to existing threads
- **Configuration Tests** (`username_avatar_test.go`, `json_mode_test.go`): Service configuration options
- **Edge Case Tests** (`long_message_test.go`, `special_characters_test.go`): Boundary conditions
