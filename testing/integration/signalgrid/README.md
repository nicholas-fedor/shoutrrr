# Signalgrid Integration Tests

This directory contains integration tests for the Signalgrid push notification service in Shoutrrr.
The tests mock HTTP requests to the Signalgrid Push API and do not make outbound network calls.

## Test Coverage

### Configuration

- URL parsing for `signalgrid://CLIENT_KEY@CHANNEL`
- Mixed-case client key and channel token preservation
- Query parameters (`title`, `type`, `critical`)
- Rejection of missing client key or channel

### Message Sending

- Basic message sending
- Title via URL query parameter
- Title via params map
- Type and critical flags
- Omission of empty title and false critical
- Empty message handling

### Error Handling

- HTTP error responses (400, 401, 403, 404, 500)
- Network errors

### API Compliance

- Endpoint `https://api.signalgrid.co/v1/push`
- POST method
- `application/x-www-form-urlencoded` content type
- Form fields `client_key`, `channel`, `body`, `title`, `type`, `critical`
- User-Agent `shoutrrr/<version>`

## Running the Tests

```bash
go test ./testing/integration/signalgrid/ -v
```
