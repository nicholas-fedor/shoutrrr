package router

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"testing/synctest"
	"text/template"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// richSenderService is a composite mock that implements types.Service and types.RichSender.
type richSenderService struct {
	*mocks.MockService
	*mocks.MockRichSender
}

// contextRichSenderService is a composite mock that implements types.Service,
// types.RichSender, and types.ContextAttachmentSender.
type contextRichSenderService struct {
	*mocks.MockService
	*mocks.MockRichSender
	*mocks.MockContextAttachmentSender
}

// failingService is a composite mock that implements types.Service.
type failingService struct {
	*mocks.MockService
}

// blockingService is a test service that implements ContextAttachmentSender
// and blocks in SendItemsContext until its context is done.
type blockingService struct{}

func (s *blockingService) GetID() string {
	return "mock-block"
}

func (s *blockingService) GetTemplate(_ string) (*template.Template, bool) {
	return nil, false
}

func (s *blockingService) Initialize(_ *url.URL, _ types.StdLogger) error {
	return nil
}

func (s *blockingService) Send(_ string, _ *types.Params) error {
	return nil
}

func (s *blockingService) SendItemsContext(ctx context.Context, _ []types.MessageItem, _ types.Params) error {
	<-ctx.Done()

	return ctx.Err()
}

func (s *blockingService) SetLogger(_ types.StdLogger) {}

func (s *blockingService) SetTemplateFile(_, _ string) error {
	return nil
}

func (s *blockingService) SetTemplateString(_, _ string) error {
	return nil
}

//nolint:paralleltest // Modifies shared serviceMap; cannot run in parallel.
func TestSendItemsDispatchesToRichSender(t *testing.T) {
	var receivedItems []types.MessageItem

	mockRichSender := mocks.NewMockRichSender(t)
	mockService := mocks.NewMockService(t)

	svc := &richSenderService{
		MockService:    mockService,
		MockRichSender: mockRichSender,
	}

	mockService.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	mockService.EXPECT().GetID().Return("mock-rich")
	mockRichSender.EXPECT().SendItems(mock.Anything, mock.Anything).RunAndReturn(func(items []types.MessageItem, params types.Params) error {
		receivedItems = items

		return nil
	})

	serviceMap["mock-rich"] = func() types.Service { return svc }
	defer delete(serviceMap, "mock-rich")

	mockLogger := mocks.NewMockStdLogger(t)

	router, err := NewWithOptions(mockLogger, types.SenderOptions{}, "mock-rich://")
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	items := []types.MessageItem{
		{Text: "hello", Level: types.Info},
		{Text: "world", Level: types.Warning},
	}

	errs := router.SendItems(items, types.Params{})

	if len(errs) != 1 {
		t.Fatalf("SendItems returned %d errors, want 1", len(errs))
	}

	if errs[0] != nil {
		t.Fatalf("SendItems returned error: %v", errs[0])
	}

	if len(receivedItems) != 2 {
		t.Errorf("SendItems received %d items, want 2", len(receivedItems))
	}
}

//nolint:paralleltest // Uses shared logger service from serviceMap.
func TestSendItemsFallsBackToPlainSend(t *testing.T) {
	mockLogger := mocks.NewMockStdLogger(t)

	mockLogger.EXPECT().Print(mock.Anything)

	router, err := NewWithOptions(mockLogger, types.SenderOptions{}, "logger://")
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	items := []types.MessageItem{
		{Text: "hello", Level: types.Info},
	}

	errs := router.SendItems(items, types.Params{})

	if len(errs) != 1 {
		t.Fatalf("SendItems returned %d errors, want 1", len(errs))
	}

	if errs[0] != nil {
		t.Fatalf("SendItems returned error: %v", errs[0])
	}
}

//nolint:paralleltest // Modifies shared serviceMap; cannot run in parallel.
func TestSendItemsWrapsErrorInTargetError(t *testing.T) {
	mockService := mocks.NewMockService(t)

	mockService.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	mockService.EXPECT().GetID().Return("mock-fail")
	mockService.EXPECT().Send(mock.Anything, mock.Anything).Return(errors.New("send failed"))

	svc := &failingService{MockService: mockService}

	serviceMap["mock-fail"] = func() types.Service { return svc }
	defer delete(serviceMap, "mock-fail")

	mockLogger := mocks.NewMockStdLogger(t)

	router, err := NewWithOptions(mockLogger, types.SenderOptions{}, "mock-fail://")
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	items := []types.MessageItem{{Text: "test"}}
	errs := router.SendItems(items, types.Params{})

	if len(errs) != 1 {
		t.Fatalf("SendItems returned %d errors, want 1", len(errs))
	}

	var targetErr *types.TargetError
	if !errors.As(errs[0], &targetErr) {
		t.Fatalf("SendItems error is not *types.TargetError: %T", errs[0])
	}

	if targetErr.URL != "mock-fail" {
		t.Errorf("TargetError.URL = %q, want %q", targetErr.URL, "mock-fail")
	}
}

//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendItemsTimeout(t *testing.T) {
	svc := &blockingService{}

	serviceMap["mock-block"] = func() types.Service { return svc }
	defer delete(serviceMap, "mock-block")

	timeout := 200 * time.Millisecond

	synctest.Test(t, func(t *testing.T) {
		router, err := NewWithOptions(nil, types.SenderOptions{Timeout: timeout}, "mock-block://")
		if err != nil {
			t.Fatalf("NewWithOptions: %v", err)
		}

		items := []types.MessageItem{{Text: "test"}}

		done := make(chan struct{})

		var errs []error

		go func() {
			defer close(done)

			errs = router.SendItems(items, types.Params{})
		}()

		synctest.Wait()

		<-done

		if len(errs) != 1 {
			t.Fatalf("SendItems returned %d errors, want 1", len(errs))
		}

		var targetErr *types.TargetError
		if !errors.As(errs[0], &targetErr) {
			t.Fatalf("SendItems error is not *types.TargetError: %T", errs[0])
		}

		if !errors.Is(targetErr.Err, ErrServiceTimeout) {
			t.Errorf("TargetError.Err = %v, want ErrServiceTimeout", targetErr.Err)
		}
	})
}

//nolint:paralleltest // Modifies shared serviceMap; cannot run in parallel.
func TestSendItemsContextPropagation(t *testing.T) {
	ctxReceived := false

	mockService := mocks.NewMockService(t)
	mockContextAttachmentSender := mocks.NewMockContextAttachmentSender(t)

	svc := &contextRichSenderService{
		MockService:                 mockService,
		MockContextAttachmentSender: mockContextAttachmentSender,
	}

	mockService.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	mockService.EXPECT().GetID().Return("mock-ctx-rich")
	mockContextAttachmentSender.EXPECT().SendItemsContext(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, items []types.MessageItem, params types.Params) error {
		if ctx != nil {
			ctxReceived = true
		}

		return nil
	})

	serviceMap["mock-ctx-rich"] = func() types.Service { return svc }
	defer delete(serviceMap, "mock-ctx-rich")

	mockLogger := mocks.NewMockStdLogger(t)

	router, err := NewWithOptions(mockLogger, types.SenderOptions{}, "mock-ctx-rich://")
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	items := []types.MessageItem{{Text: "hello"}}
	errs := router.SendItems(items, types.Params{})

	if len(errs) != 1 {
		t.Fatalf("SendItems returned %d errors, want 1", len(errs))
	}

	if errs[0] != nil {
		t.Fatalf("SendItems returned error: %v", errs[0])
	}

	if !ctxReceived {
		t.Error("context was not propagated to SendItemsContext")
	}
}

//nolint:paralleltest // Simple isolated test, no shared state.
func TestTargetErrorUnwrap(t *testing.T) {
	inner := errors.New("inner error")
	targetErr := &types.TargetError{URL: "test://", Err: inner}

	if !errors.Is(targetErr, inner) {
		t.Error("errors.Is should find the wrapped error")
	}

	unwrapped := targetErr.Unwrap()
	if unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}
}
