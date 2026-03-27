//go:build js && wasm

package main

import (
	"encoding/json"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListServicesJSON(t *testing.T) {
	t.Run("returns valid JSON array", func(t *testing.T) {
		result := listServicesJSON()

		var schemes []string

		err := json.Unmarshal([]byte(result), &schemes)
		require.NoError(t, err)
		assert.NotEmpty(t, schemes)
	})

	t.Run("contains known services", func(t *testing.T) {
		result := listServicesJSON()

		var schemes []string

		err := json.Unmarshal([]byte(result), &schemes)
		require.NoError(t, err)

		assert.Contains(t, schemes, "discord")
		assert.Contains(t, schemes, "slack")
		assert.Contains(t, schemes, "ntfy")
		assert.Contains(t, schemes, "generic")
		assert.Contains(t, schemes, "logger")
	})

	t.Run("matches router.ListServices", func(t *testing.T) {
		r := router.ServiceRouter{}
		expected := r.ListServices()

		result := listServicesJSON()

		var schemes []string

		err := json.Unmarshal([]byte(result), &schemes)
		require.NoError(t, err)
		assert.ElementsMatch(t, schemes, expected)
	})
}

func TestConfigSchemaJSON(t *testing.T) {
	t.Run("returns valid schema for discord", func(t *testing.T) {
		result := configSchemaJSON("discord")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)
		assert.Equal(t, "discord", schema.Service)
		assert.Equal(t, "discord", schema.Scheme)
		assert.NotEmpty(t, schema.Fields)
	})

	t.Run("returns valid schema for ntfy", func(t *testing.T) {
		result := configSchemaJSON("ntfy")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)
		assert.Equal(t, "ntfy", schema.Service)
	})

	t.Run("includes webhookURL field for generic service", func(t *testing.T) {
		result := configSchemaJSON("generic")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)

		var found bool

		for _, field := range schema.Fields {
			if field.Name == "WebhookURL" && field.Required && field.Type == "string" {
				found = true

				break
			}
		}

		assert.True(t, found, "expected to find WebhookURL field with Required=true and Type=string")
	})

	t.Run("returns error for invalid service", func(t *testing.T) {
		result := configSchemaJSON("nonexistent")

		var errResp errorResult

		err := json.Unmarshal([]byte(result), &errResp)
		require.NoError(t, err)
		assert.NotEmpty(t, errResp.Error)
	})

	t.Run("handles logger service with no fields", func(t *testing.T) {
		result := configSchemaJSON("logger")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)
		assert.Equal(t, "logger", schema.Service)
		assert.Empty(t, schema.Fields)
	})
}
