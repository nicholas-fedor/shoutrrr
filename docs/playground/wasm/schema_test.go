//go:build js && wasm

package main

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

// TestListServicesJSON verifies that the services are listed as a JSON array
// that matches the router's list.
func TestListServicesJSON(t *testing.T) {
	t.Parallel()

	t.Run("returns valid JSON array", func(t *testing.T) {
		t.Parallel()

		result := listServicesJSON()

		var schemes []string

		err := json.Unmarshal([]byte(result), &schemes)
		require.NoError(t, err)
		assert.NotEmpty(t, schemes)
	})

	t.Run("contains known services", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

		r := router.ServiceRouter{}
		expected := r.ListServices()

		result := listServicesJSON()

		var schemes []string

		err := json.Unmarshal([]byte(result), &schemes)
		require.NoError(t, err)
		assert.ElementsMatch(t, schemes, expected)
	})
}

// TestConfigSchemaJSON verifies the config schemas of several services,
// including enum names, the generic webhook field, and a service without
// fields, and the error for an unknown service.
func TestConfigSchemaJSON(t *testing.T) {
	t.Parallel()

	t.Run("returns valid schema for discord", func(t *testing.T) {
		t.Parallel()

		result := configSchemaJSON("discord")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)
		assert.Equal(t, "discord", schema.Service)
		assert.Equal(t, "discord", schema.Scheme)
		assert.NotEmpty(t, schema.Fields)
	})

	t.Run("returns valid schema for ntfy", func(t *testing.T) {
		t.Parallel()

		result := configSchemaJSON("ntfy")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)
		assert.Equal(t, "ntfy", schema.Service)
	})

	t.Run("lists the names of an enum field only", func(t *testing.T) {
		t.Parallel()

		fields := map[string]fieldSchema{}

		for _, service := range []string{"ntfy", "gotify"} {
			var schema configSchema

			require.NoError(t, json.Unmarshal([]byte(configSchemaJSON(service)), &schema))

			for _, field := range schema.Fields {
				fields[service+"."+field.Name] = field
			}
		}

		// ntfy's priority is an enum, while Gotify's is a plain int.
		assert.Equal(t, "enum", fields["ntfy.Priority"].Type)
		assert.Equal(t, []string{"Min", "Low", "Default", "High", "Max"}, fields["ntfy.Priority"].EnumValues)
		assert.Equal(t, "int", fields["gotify.Priority"].Type)
		assert.Nil(t, fields["gotify.Priority"].EnumValues)
	})

	t.Run("includes webhookURL field for generic service", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

		result := configSchemaJSON("nonexistent")

		var errResp errorResult

		err := json.Unmarshal([]byte(result), &errResp)
		require.NoError(t, err)
		assert.NotEmpty(t, errResp.Error)
	})

	t.Run("handles logger service with no fields", func(t *testing.T) {
		t.Parallel()

		result := configSchemaJSON("logger")

		var schema configSchema

		err := json.Unmarshal([]byte(result), &schema)
		require.NoError(t, err)
		assert.Equal(t, "logger", schema.Service)
		assert.Empty(t, schema.Fields)
	})
}
