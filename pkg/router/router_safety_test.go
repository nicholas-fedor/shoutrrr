package router

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

const routerTestSecret = "SECRETrouterTOKEN"

var errConverterFailed = errors.New("converter failed")

// locateWithoutPanic calls Locate and fails the test instead of crashing on a panic.
func locateWithoutPanic(t *testing.T, r *ServiceRouter, rawURL string) error {
	t.Helper()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("Locate(%q) panicked: %v", rawURL, p)
		}
	}()

	_, err := r.Locate(rawURL)

	return err
}

//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestCustomURLConversionFailure(t *testing.T) {
	tests := []struct {
		name      string
		converted error
		wantCause error
	}{
		{name: "converter error", converted: errConverterFailed, wantCause: errConverterFailed},
		{name: "converter returns no URL", converted: nil, wantCause: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := mocks.NewMockCustomURLService(t)
			service.EXPECT().GetServiceURLFromCustom(mock.Anything).Return(nil, tt.converted)
			registerService(t, "contracttest", service)

			err := locateWithoutPanic(t, &ServiceRouter{}, "contracttest+https://"+routerTestSecret+"@hooks.example.invalid/hook")

			if !errors.Is(err, ErrCustomURLConversion) {
				t.Fatalf("error = %v, want ErrCustomURLConversion", err)
			}

			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Errorf("error = %v, want it to wrap %v", err, tt.wantCause)
			}

			if strings.Contains(err.Error(), routerTestSecret) {
				t.Errorf("error leaks the custom URL: %v", err)
			}
		})
	}
}

func TestGenericCustomURLConversionFailure(t *testing.T) {
	t.Parallel()

	err := locateWithoutPanic(t, &ServiceRouter{}, "generic+https://hooks.example.invalid/hook?disabletls=maybe")

	if !errors.Is(err, ErrCustomURLConversion) {
		t.Fatalf("error = %v, want ErrCustomURLConversion", err)
	}
}

func TestParseErrorOmitsURL(t *testing.T) {
	t.Parallel()

	r := &ServiceRouter{}

	_, _, err := r.ExtractServiceName("discord://" + routerTestSecret + "@123456/%zz")

	if !errors.Is(err, ErrParseURLFailed) {
		t.Fatalf("error = %v, want ErrParseURLFailed", err)
	}

	if strings.Contains(err.Error(), routerTestSecret) {
		t.Errorf("parse error leaks the URL: %v", err)
	}
}

func TestCustomURLLogsOmitURL(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	r := &ServiceRouter{logger: log.New(&logs, "", 0)}

	err := locateWithoutPanic(t, r,
		"generic+https://hooks.example.invalid/hook?token="+routerTestSecret+"&@Authorization=Bearer%20"+routerTestSecret)
	if err != nil {
		t.Fatalf("Locate failed: %v", err)
	}

	if strings.Contains(logs.String(), routerTestSecret) {
		t.Errorf("router logs leak the custom URL:\n%s", logs.String())
	}

	if !strings.Contains(logs.String(), "generic") {
		t.Errorf("router logs should still name the service:\n%s", logs.String())
	}
}
