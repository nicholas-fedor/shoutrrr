//go:build js && wasm

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	t.Run("sendString", func(t *testing.T) {
		tests := []struct {
			name     string
			url      string
			message  string
			validate func(t *testing.T, result string)
		}{
			{
				name:    "returns success for valid logger URL",
				url:     "logger://",
				message: "test message",
				validate: func(t *testing.T, result string) {
					assert.Equal(t, `{"success":true}`, result)
				},
			},
			{
				name:    "returns error for invalid URL",
				url:     "invalid://url",
				message: "test message",
				validate: func(t *testing.T, result string) {
					var errResp errorResult
					err := json.Unmarshal([]byte(result), &errResp)
					require.NoError(t, err)
					assert.NotEmpty(t, errResp.Error)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := sendString(tt.url, tt.message)
				tt.validate(t, result)
			})
		}
	})
}
