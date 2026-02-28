package mqtt_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// publishMethodName is the constant for the Publish method to avoid string duplication.
const publishMethodName = "Publish"

// createTestService creates an MQTT service instance configured for testing.
// If a mockConnectionManager is provided, it will be injected into the service
// to bypass the normal connection initialization.
func createTestService(
	t *testing.T,
	mqttURL string,
	mockConnectionManager ...mqtt.ConnectionManager,
) *mqtt.Service {
	t.Helper()

	service := &mqtt.Service{}

	parsedURL, err := url.Parse(mqttURL)
	require.NoError(t, err)

	err = service.Initialize(parsedURL, &mockLogger{})
	require.NoError(t, err)

	// Inject mock connection manager if provided
	if len(mockConnectionManager) > 0 && mockConnectionManager[0] != nil {
		service.SetConnectionManager(mockConnectionManager[0])
	}

	return service
}

// mockLogger is a simple logger implementation for testing.
type mockLogger struct{}

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}

// MockConnectionManager is a testify mock that implements the ConnectionManager interface.
type MockConnectionManager struct {
	mock.Mock
}

// AwaitConnection mocks waiting for the MQTT connection to be established.
func (m *MockConnectionManager) AwaitConnection(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

// Publish mocks sending an MQTT message to the broker.
func (m *MockConnectionManager) Publish(
	ctx context.Context,
	publish *paho.Publish,
) (*paho.PublishResponse, error) {
	args := m.Called(ctx, publish)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*paho.PublishResponse), args.Error(1)
}

// Disconnect mocks gracefully closing the MQTT connection.
func (m *MockConnectionManager) Disconnect(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

// createMockPublishResponse creates a mock publish response with the given reason code.
func createMockPublishResponse(reasonCode byte) *paho.PublishResponse {
	return &paho.PublishResponse{
		ReasonCode: reasonCode,
		Properties: &paho.PublishResponseProperties{},
	}
}

// createMockPublishResponseWithReason creates a mock publish response with reason code and string.
func createMockPublishResponseWithReason(
	reasonCode byte,
	reasonString string,
) *paho.PublishResponse {
	return &paho.PublishResponse{
		ReasonCode: reasonCode,
		Properties: &paho.PublishResponseProperties{
			ReasonString: reasonString,
		},
	}
}

// createTestParams creates test parameters with the given key-value pairs.
func createTestParams(pairs ...string) *types.Params {
	params := make(types.Params)

	for i := 0; i < len(pairs); i += 2 {
		if i+1 < len(pairs) {
			params[pairs[i]] = pairs[i+1]
		}
	}

	return &params
}

// assertPublishCalled asserts that Publish was called with the expected topic.
func assertPublishCalled(t *testing.T, mockManager *MockConnectionManager, expectedTopic string) {
	t.Helper()

	found := false

	for _, call := range mockManager.Calls {
		if call.Method == publishMethodName {
			publish := call.Arguments[1].(*paho.Publish)
			if publish.Topic == expectedTopic {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf("Expected Publish call with topic %q, but no matching call found", expectedTopic)
	}
}

// assertPublishPayload asserts that Publish was called with the expected payload.
func assertPublishPayload(
	t *testing.T,
	mockManager *MockConnectionManager,
	expectedPayload string,
) {
	t.Helper()

	found := false

	for _, call := range mockManager.Calls {
		if call.Method == publishMethodName {
			publish := call.Arguments[1].(*paho.Publish)
			if string(publish.Payload) == expectedPayload {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf(
			"Expected Publish call with payload %q, but no matching call found",
			expectedPayload,
		)
	}
}

// assertPublishQoS asserts that Publish was called with the expected QoS level.
func assertPublishQoS(t *testing.T, mockManager *MockConnectionManager, expectedQoS byte) {
	t.Helper()

	found := false

	for _, call := range mockManager.Calls {
		if call.Method == publishMethodName {
			publish := call.Arguments[1].(*paho.Publish)
			if publish.QoS == expectedQoS {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf("Expected Publish call with QoS %d, but no matching call found", expectedQoS)
	}
}

// assertPublishRetained asserts that Publish was called with the expected retained flag.
func assertPublishRetained(
	t *testing.T,
	mockManager *MockConnectionManager,
	expectedRetained bool,
) {
	t.Helper()

	found := false

	for _, call := range mockManager.Calls {
		if call.Method == publishMethodName {
			publish := call.Arguments[1].(*paho.Publish)
			if publish.Retain == expectedRetained {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf(
			"Expected Publish call with Retain=%v, but no matching call found",
			expectedRetained,
		)
	}
}

// getPublishCall retrieves the publish struct from the first Publish call.
// Returns nil if no Publish call was made.
func getPublishCall(mockManager *MockConnectionManager) *paho.Publish {
	for _, call := range mockManager.Calls {
		if call.Method == publishMethodName {
			if publish, ok := call.Arguments[1].(*paho.Publish); ok {
				return publish
			}
		}
	}

	return nil
}
