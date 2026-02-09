# MQTT Integration Tests

This directory contains integration tests for the MQTT service in Shoutrrr.
The tests validate MQTT functionality by mocking the connection manager, ensuring feature parity with MQTT v5 protocol.

## Test Coverage

### Configuration Options

- URL parsing for `mqtt://` and `mqtts://` schemes
- Default ports (1883 for mqtt, 8883 for mqtts)
- Query parameters (qos, retained, clientid, cleansession, disabletls, disabletlsverification)
- Username/password authentication from URL
- Invalid URL handling (missing topic)
- Invalid QoS values

### Message Publishing

- Basic message publish
- Publish with different QoS levels (0, 1, 2)
- Publish with retained flag
- Publish with params override
- Multiple messages
- Topic variations (simple, nested, deep paths)

### Error Handling

- Connection errors
- Publish errors with reason codes
- Timeout errors
- Connection not initialized error
- Invalid QoS errors
- Disconnect errors

### Timeout Scenarios

- Publish timeout (10s)
- Disconnect timeout (5s)
- Context cancellation handling
- Fast operation completion before timeout

### Edge Cases

- Empty message
- Very long message (1MB+)
- Special characters in topic
- Unicode in message (emoji, Chinese, Japanese, Arabic, Russian)
- Newlines and tabs in message
- JSON and XML payloads
- HTML entities
- Whitespace-only messages
- Binary-like data
- Concurrent message publishing
- Deep topic paths

## Running the Tests

### Mocked Integration Tests (Default)

Run all integration tests with mocked MQTT connection:

```bash
go test ./testing/integration/mqtt/ -v
```

Or run specific test files:

```bash
# Run specific test categories
go test ./testing/integration/mqtt/ -run TestConfig -v
go test ./testing/integration/mqtt/ -run TestPublish -v
go test ./testing/integration/mqtt/ -run TestErrors -v
go test ./testing/integration/mqtt/ -run TestTimeout -v
go test ./testing/integration/mqtt/ -run TestEdgeCases -v
```

## Test Structure

The test suite is organized as a flat directory structure with individual test files for each feature category:

```bash
testing/integration/mqtt/
├── utils_test.go        # Helper functions and mock implementations
├── config_test.go       # Configuration and URL parsing tests
├── publish_test.go      # Message publishing tests
├── errors_test.go       # Error handling tests
├── timeout_test.go      # Timeout scenario tests
├── edge_cases_test.go   # Edge case and boundary condition tests
└── README.md            # This documentation
```

## Mock Architecture

### MockConnectionManager

The `MockConnectionManager` implements the `mqtt.ConnectionManager` interface using testify mock:

```go
type MockConnectionManager struct {
    mock.Mock
}
```

Methods mocked:

- `AwaitConnection(ctx context.Context) error`
- `Publish(ctx context.Context, publish *paho.Publish) (*paho.PublishResponse, error)`
- `Disconnect(ctx context.Context) error`

### Helper Functions

- `createTestService(t *testing.T, url string, mockManager ...ConnectionManager) *mqtt.Service`
- `createMockPublishResponse(reasonCode byte) *paho.PublishResponse`
- `createMockPublishResponseWithReason(reasonCode byte, reasonString string) *paho.PublishResponse`
- `createTestParams(pairs ...string) *types.Params`
- `assertPublishCalled(t *testing.T, mockManager *MockConnectionManager, expectedTopic string)`
- `assertPublishPayload(t *testing.T, mockManager *MockConnectionManager, expectedPayload string)`
- `assertPublishQoS(t *testing.T, mockManager *MockConnectionManager, expectedQoS byte)`
- `assertPublishRetained(t *testing.T, mockManager *MockConnectionManager, expectedRetained bool)`
- `getPublishCall(mockManager *MockConnectionManager) *paho.Publish`

## MQTT v5 Reason Codes

The tests cover various MQTT v5 reason codes:

| Code | Name                          | Type    |
|------|-------------------------------|---------|
| 0x00 | Success                       | Success |
| 0x01 | Granted QoS 1                 | Success |
| 0x02 | Granted QoS 2                 | Success |
| 0x10 | No matching subscribers       | Warning |
| 0x80 | Unspecified error             | Failure |
| 0x81 | Malformed Packet              | Failure |
| 0x82 | Protocol Error                | Failure |
| 0x83 | Implementation specific error | Failure |
| 0x84 | Unsupported Protocol Version  | Failure |
| 0x85 | Client Identifier not valid   | Failure |
| 0x86 | Bad User Name or Password     | Failure |
| 0x87 | Not authorized                | Failure |

Codes >= 0x80 are treated as failures and return an error.
Codes < 0x80 are treated as success/warnings and do not return an error.

## Test Patterns

All tests follow the synctest pattern for time-based testing:

```go
func TestFeature(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        // Test code
    })
}
```

Mock expectations are always verified:

```go
mockManager.AssertExpectations(t)
```

## Constants

The MQTT service uses the following timeout constants:

- `publishTimeout` = 10 seconds
- `disconnectTimeout` = 5 seconds
- `keepAliveInterval` = 20 seconds
- `sessionExpiryInterval` = 60 seconds

## Related Files

- MQTT Service Implementation: `pkg/services/push/mqtt/mqtt.go`
- MQTT Configuration: `pkg/services/push/mqtt/mqtt_config.go`
- MQTT Errors: `pkg/services/push/mqtt/mqtt_errors.go`
- Connection Manager Interface: `pkg/services/push/mqtt/mqtt_connection_manager.go`
