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

Run one group of tests:

```bash
# Test basic functionality
go test ./testing/e2e/discord/ -v -run 'TestDiscordSend/basic'

# Test embed features
go test ./testing/e2e/discord/ -v -run 'TestDiscordSendItems/embed'

# Test file attachments
go test ./testing/e2e/discord/ -v -run 'TestDiscordSendItems/file'
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
├── suite_test.go                 # Test setup, configuration, and .env file loading
├── send_test.go                  # Plain text messages
├── send_items_test.go            # Rich embeds and file attachments
└── thread_test.go                # Thread functionality
```

### Test Organization

Each test file covers one kind of send, with a subtest per feature:

- **Send Tests** (`send_test.go`): Basic text, special characters, custom username and avatar, JSON mode, and long messages
- **Send Items Tests** (`send_items_test.go`): Embed colors, timestamps, fields, images, author information, multiple embeds, file attachments, and all of these combined
- **Thread Tests** (`thread_test.go`): Posting messages to existing threads
