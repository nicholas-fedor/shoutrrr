//go:build js && wasm

package main

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSBindings(t *testing.T) {
	t.Run("getServices", func(t *testing.T) {
		result := getServices(js.Value{}, nil)
		str, ok := result.(string)
		require.True(t, ok, "expected string result")
		assert.Contains(t, str, "discord")
	})

	t.Run("getConfigSchema", func(t *testing.T) {
		t.Run("returns error when no args provided", func(t *testing.T) {
			result := getConfigSchema(js.Value{}, nil)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})

		t.Run("returns schema for discord", func(t *testing.T) {
			args := []js.Value{js.ValueOf("discord")}
			result := getConfigSchema(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "discord")
			assert.Contains(t, str, "WebhookID")
		})

		t.Run("returns error for invalid service", func(t *testing.T) {
			args := []js.Value{js.ValueOf("nonexistent")}
			result := getConfigSchema(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})
	})

	t.Run("parseURL", func(t *testing.T) {
		t.Run("returns error when no args provided", func(t *testing.T) {
			result := parseURL(js.Value{}, nil)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})

		t.Run("parses discord URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("discord://token@123456789")}
			result := parseURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "discord")
			assert.Contains(t, str, "config")
		})

		t.Run("parses ntfy URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("ntfy://ntfy.sh/mytopic")}
			result := parseURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "ntfy")
		})

		t.Run("returns error for invalid URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("not-a-valid-url")}
			result := parseURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})
	})

	t.Run("generateURL", func(t *testing.T) {
		t.Run("returns error when insufficient args", func(t *testing.T) {
			result := generateURL(js.Value{}, nil)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})

		t.Run("generates discord URL", func(t *testing.T) {
			args := []js.Value{
				js.ValueOf("discord"),
				js.ValueOf(`{"WebhookID":"123456789","Token":"mytoken"}`),
			}
			result := generateURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "url")
			assert.Contains(t, str, "discord://")
		})

		t.Run("returns error for invalid service", func(t *testing.T) {
			args := []js.Value{
				js.ValueOf("nonexistent"),
				js.ValueOf("{}"),
			}
			result := generateURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})

		t.Run("returns error for invalid JSON", func(t *testing.T) {
			args := []js.Value{
				js.ValueOf("discord"),
				js.ValueOf("not-json"),
			}
			result := generateURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})
	})

	t.Run("validateURL", func(t *testing.T) {
		t.Run("returns error when no args provided", func(t *testing.T) {
			result := validateURL(js.Value{}, nil)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})

		t.Run("validates discord URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("discord://token@123456789")}
			result := validateURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "valid")
		})

		t.Run("validates ntfy URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("ntfy://ntfy.sh/mytopic")}
			result := validateURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "valid")
		})

		t.Run("returns error for invalid URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("not-valid")}
			result := validateURL(js.Value{}, args)
			str, ok := result.(string)
			require.True(t, ok, "expected string result")
			assert.Contains(t, str, "error")
		})
	})

	t.Run("send", func(t *testing.T) {
		t.Run("returns rejected Promise for insufficient args", func(t *testing.T) {
			result := send(js.Value{}, nil)
			promise, ok := result.(js.Value)
			require.True(t, ok, "expected js.Value result")
			assert.Equal(t, "Promise", promise.Get("constructor").Get("name").String())

			// Use channel/timeout pattern because the Promise rejection is
			// dispatched asynchronously by the Go WASM runtime.
			done := make(chan string, 1)

			var catchHandler js.Func
			catchHandler = js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
				catchHandler.Release()
				done <- args[0].String()

				return nil
			})

			promise.Call("catch", catchHandler)

			select {
			case errMsg := <-done:
				assert.Contains(t, errMsg, "missing arguments")
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for Promise rejection")
			}
		})

		t.Run("returns rejected Promise for invalid URL", func(t *testing.T) {
			args := []js.Value{js.ValueOf("not-a-valid-url"), js.ValueOf("test message")}
			result := send(js.Value{}, args)
			promise, ok := result.(js.Value)
			require.True(t, ok, "expected js.Value result")
			assert.Equal(t, "Promise", promise.Get("constructor").Get("name").String())

			// Async rejection: wait for goroutine via channel.
			done := make(chan string, 1)

			var catchHandler js.Func
			catchHandler = js.FuncOf(func(_ js.Value, catchArgs []js.Value) interface{} {
				catchHandler.Release()
				done <- catchArgs[0].String()

				return nil
			})

			promise.Call("catch", catchHandler)

			select {
			case errMsg := <-done:
				assert.Contains(t, errMsg, "error")
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for Promise rejection")
			}
		})

		t.Run("returns rejected Promise for empty message", func(t *testing.T) {
			args := []js.Value{js.ValueOf("logger://"), js.ValueOf("")}
			result := send(js.Value{}, args)
			promise, ok := result.(js.Value)
			require.True(t, ok, "expected js.Value result")
			assert.Equal(t, "Promise", promise.Get("constructor").Get("name").String())

			done := make(chan string, 1)

			var catchHandler js.Func
			catchHandler = js.FuncOf(func(_ js.Value, catchArgs []js.Value) interface{} {
				catchHandler.Release()
				done <- catchArgs[0].String()

				return nil
			})

			promise.Call("catch", catchHandler)

			select {
			case errMsg := <-done:
				assert.Contains(t, errMsg, "error")
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for Promise rejection")
			}
		})
	})
}
