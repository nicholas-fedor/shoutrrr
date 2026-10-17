package smtp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEncMethodString verifies the name of each encryption method, which the
// URL uses as the encryption value.
func TestEncMethodString(t *testing.T) {
	t.Parallel()

	// tests lists methods with their names.
	tests := []struct {
		want   string
		method encMethod
	}{
		{method: EncNone, want: "None"},
		{method: EncExplicitTLS, want: "ExplicitTLS"},
		{method: EncImplicitTLS, want: "ImplicitTLS"},
		{method: EncAuto, want: "Auto"},
		{method: encMethod(99), want: "Invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.method.String())
		})
	}

	assert.Equal(t, &encMethodVals{
		None: EncNone, ExplicitTLS: EncExplicitTLS, ImplicitTLS: EncImplicitTLS, Auto: EncAuto, Enum: EncMethods.Enum,
	}, EncMethods)
}

// TestUseImplicitTLS verifies that implicit TLS is used for ImplicitTLS on
// any port and for Auto on port 465 only.
func TestUseImplicitTLS(t *testing.T) {
	t.Parallel()

	// tests lists methods and ports with whether each should use implicit TLS.
	tests := []struct {
		method encMethod
		port   uint16
		want   bool
	}{
		{method: EncNone, port: ImplicitTLSPort},
		{method: EncNone, port: 587},
		{method: EncExplicitTLS, port: ImplicitTLSPort},
		{method: EncExplicitTLS, port: 587},
		{method: EncImplicitTLS, port: ImplicitTLSPort, want: true},
		{method: EncImplicitTLS, port: 25, want: true},
		{method: EncAuto, port: ImplicitTLSPort, want: true},
		{method: EncAuto, port: 587},
		{method: EncAuto, port: 25},
		{method: encMethod(99), port: ImplicitTLSPort},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v on %d", tt.method, tt.port), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, useImplicitTLS(tt.method, tt.port))
		})
	}
}
