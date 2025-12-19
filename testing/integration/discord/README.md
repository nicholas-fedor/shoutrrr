# Discord Integration Tests

This directory contains integration tests for the Discord service in Shoutrrr. The tests validate Discord webhook functionality by mocking HTTP requests to Discord's API, ensuring feature parity with Discord's webhook API.

## Test Coverage

### Content Field Support

- Plain text messages with various scenarios (special characters, long messages, empty/whitespace handling)

### Enhanced Embed Features

- Author information (name, URL, icon URL)
- Image and thumbnail URLs
- Custom fields array
- Timestamps
- Multiple embeds with different features
- Message levels with color coding (Error, Warning, Info, Debug)

### File Attachments

- Single and multiple file attachments
- Large file handling (1MB+)
- Different file types (PDF, PNG, JS, CSV, etc.)
- Special characters in filenames
- Rich embeds combined with file attachments

### Thread Creation and Messaging

- Creating new threads with `thread_name`
- Posting to existing threads with `thread_id`
- Whitespace handling in thread IDs
- Embed messages in threads
- File attachments in threads

### Configuration Options

- Custom username and avatar overrides
- Color customization (default and per-level colors)
- JSON mode for raw payloads
- Message chunking for long content
- Split lines option for multi-line messages

### External API Interaction

- HTTP request construction and sending
- Response handling from mocked Discord API
- Service behavior based on external API responses
- Error handling for network and API failures

### Error Handling and Edge Cases

- HTTP error responses (400, etc.)
- Network errors and invalid configurations
- Malformed JSON handling
- Empty file attachments
- Very long filenames
- Messages exceeding Discord's limits

## Running the Tests

### Mocked Integration Tests (Default)

Run all integration tests with mocked Discord API responses:

```bash
go test ./testing/integration/discord/ -v
```

Or run specific test files:

```bash
# Run specific test categories
go test ./testing/integration/discord/ -run TestAPICompliance -v
go test ./testing/integration/discord/ -run TestContent -v
go test ./testing/integration/discord/ -run TestEdgeCases -v
go test ./testing/integration/discord/ -run TestEmbeds -v
go test ./testing/integration/discord/ -run TestThreads -v
go test ./testing/integration/discord/ -run TestConfig -v
go test ./testing/integration/discord/ -run TestFiles -v
go test ./testing/integration/discord/ -run TestErrors -v
```

## Test Structure

The test suite is organized as a flat directory structure with individual test files for each feature category:

```bash
testing/integration/discord/
├── api_compliance_test.go # API compliance and webhook specification validation
├── config_test.go         # Configuration options (username, avatar, colors, JSON mode)
├── content_test.go        # Plain text message functionality
├── embeds_test.go         # Embed features (author, image, thumbnail, fields, timestamps)
├── errors_test.go         # Error handling and edge cases
├── files_test.go          # File attachment functionality
├── http_test.go           # HTTP-related tests
├── integration_suite_test.go # Test suite setup and configuration
├── threads_test.go        # Thread creation and messaging
└── utils_test.go          # Shared test utilities and helpers
```

### Test Organization

Each test file contains independent black-box tests for specific Discord service behaviors. Tests validate that the service correctly interacts with external APIs without inspecting internal data structures:

- **API Compliance Tests** (`api_compliance_test.go`): Service behavior with different Discord API response scenarios
- **Content Tests** (`content_test.go`): Service handling of various message types and external API responses
- **Embed Tests** (`embeds_test.go`): Service processing of rich embed messages with mocked API responses
- **Thread Tests** (`threads_test.go`): Service behavior for thread creation and messaging with external API interaction
- **Config Tests** (`config_test.go`): Service configuration application in external API requests
- **File Tests** (`files_test.go`): Service handling of file attachments with mocked multipart responses
- **Error Tests** (`errors_test.go`): Service error handling when external APIs return failures
- **Edge Cases Tests** (`edge_cases_test.go`): Service behavior under unusual conditions and API responses
