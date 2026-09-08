# Home Assistant Integration Tests

This directory contains integration tests for the Home Assistant notification service in Shoutrrr.
The tests mock HTTP requests to the Home Assistant REST API and do not make outbound network calls.

## Test Coverage

### Configuration

- URL parsing for `homeassistant://TOKEN@host`
- Query parameters (`title`, `nid`, `disabletls`, `service`, `targets`)
- Rejection of missing token or host

### Message Sending

- Basic persistent notification
- Title and notification ID
- Title via params map
- Notify-platform actions without `notification_id`
- Rejection of empty messages

### Error Handling

- HTTP error responses (400, 401, 404, 500)
- Network errors

### API Compliance

- Endpoint `POST /api/services/persistent_notification/create`
- JSON content type
- `Authorization: Bearer` header
- User-Agent `shoutrrr/<version>`
- Implied HTTP port 8123 when TLS is disabled
- Reverse-proxy path prefix

## Running the Tests

```bash
go test ./testing/integration/homeassistant/ -v
```
