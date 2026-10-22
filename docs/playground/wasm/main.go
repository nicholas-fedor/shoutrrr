//go:build js && wasm

package main

import (
	"encoding/json/v2"
	"fmt"
	"syscall/js"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// pairArgs is the number of arguments generateURL and send require.
const pairArgs = 2

// main registers JavaScript functions on the global object and blocks
// to keep the WASM runtime alive. Each registered function delegates
// to a pure logic function that has no syscall/js dependency.
func main() {
	js.Global().Set("shoutrrrGetServices", js.FuncOf(getServices))
	js.Global().Set("shoutrrrGetConfigSchema", js.FuncOf(getConfigSchema))
	js.Global().Set("shoutrrrParseURL", js.FuncOf(parseURL))
	js.Global().Set("shoutrrrGenerateURL", js.FuncOf(generateURL))
	js.Global().Set("shoutrrrValidateURL", js.FuncOf(validateURL))
	js.Global().Set("shoutrrrSend", js.FuncOf(send))

	<-make(chan struct{})
}

// getServices returns all registered service schemes as a JSON array.
func getServices(_ js.Value, _ []js.Value) any {
	return listServicesJSON()
}

// getConfigSchema returns the configuration schema for args[0] (service name).
func getConfigSchema(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalErrorStr("missing service name argument")
	}

	return configSchemaJSON(args[0].String())
}

// parseURL parses args[0] (Shoutrrr URL) and returns service + config values.
func parseURL(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalErrorStr("missing URL argument")
	}

	return parseURLString(args[0].String())
}

// generateURL builds a Shoutrrr URL from args[0] (service) and args[1] (config JSON).
func generateURL(_ js.Value, args []js.Value) any {
	if len(args) < pairArgs {
		return marshalErrorStr("missing arguments: expected service name and config JSON")
	}

	return generateURLString(args[0].String(), args[1].String())
}

// validateURL validates args[0] (Shoutrrr URL) and returns success or error.
func validateURL(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalErrorStr("missing URL argument")
	}

	return validateURLString(args[0].String())
}

// send returns a Promise that sends args[0] (URL) with args[1] (message).
// The send runs in a goroutine to avoid blocking the JavaScript event loop.
func send(_ js.Value, args []js.Value) any {
	if len(args) < pairArgs {
		errJSON := marshalErrorStr("missing arguments: expected URL and message")

		return js.Global().Get("Promise").Call("reject", js.ValueOf(errJSON))
	}

	rawURL := args[0].String()
	message := args[1].String()

	var promiseHandler js.Func

	promiseHandler = js.FuncOf(func(_ js.Value, promiseArgs []js.Value) any {
		resolve := promiseArgs[0]
		reject := promiseArgs[1]

		go func() {
			defer promiseHandler.Release()

			// Recover from panics in sendString to prevent hanging promises.
			defer func() {
				if r := recover(); r != nil {
					errMsg := fmt.Sprintf("send panicked: %v", r)
					reject.Invoke(marshalErrorStr(errMsg))
				}
			}()

			result := sendString(rawURL, message)

			var errResp errorResult
			if err := json.Unmarshal([]byte(result), &errResp, jsonopts.Default); err != nil {
				// Malformed response from sendString — reject with context.
				reject.Invoke(marshalErrorStr(
					fmt.Sprintf("invalid response from send: %v (raw: %s)", err, result),
				))

				return
			}

			if errResp.Error != "" {
				reject.Invoke(result)
			} else {
				resolve.Invoke(result)
			}
		}()

		return nil
	})

	return js.Global().Get("Promise").New(promiseHandler)
}
