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

// main registers the playground's functions on the JavaScript global object
// and blocks forever.
//
// Blocking keeps the WASM runtime alive for the registered functions. Each
// function checks its arguments and calls a function that does not depend on
// syscall/js.
func main() {
	js.Global().Set("shoutrrrGetServices", js.FuncOf(getServices))
	js.Global().Set("shoutrrrGetConfigSchema", js.FuncOf(getConfigSchema))
	js.Global().Set("shoutrrrParseURL", js.FuncOf(parseURL))
	js.Global().Set("shoutrrrGenerateURL", js.FuncOf(generateURL))
	js.Global().Set("shoutrrrValidateURL", js.FuncOf(validateURL))
	js.Global().Set("shoutrrrSend", js.FuncOf(send))

	<-make(chan struct{})
}

// getServices returns the schemes of every service as a JSON array.
//
// It is registered as shoutrrrGetServices.
//
// Parameters:
//   - js.Value: unused, the JavaScript this value.
//   - []js.Value: unused, since the function takes no arguments.
//
// Returns:
//   - any: the JSON array as a string.
func getServices(_ js.Value, _ []js.Value) any {
	return listServicesJSON()
}

// getConfigSchema returns the config schema of a service as JSON.
//
// It is registered as shoutrrrGetConfigSchema.
//
// Parameters:
//   - js.Value: unused, the JavaScript this value.
//   - args: the service name.
//
// Returns:
//   - any: the configSchema, or an errorResult when the name is missing or no
//     service has it, as a JSON string.
func getConfigSchema(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalErrorStr("missing service name argument")
	}

	return configSchemaJSON(args[0].String())
}

// parseURL reads a Shoutrrr URL into its service and config values.
//
// It is registered as shoutrrrParseURL.
//
// Parameters:
//   - js.Value: unused, the JavaScript this value.
//   - args: the Shoutrrr URL.
//
// Returns:
//   - any: the parseResult, or an errorResult when the URL is missing or is
//     rejected, as a JSON string.
func parseURL(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalErrorStr("missing URL argument")
	}

	return parseURLString(args[0].String())
}

// generateURL builds a Shoutrrr URL from a service name and config values.
//
// It is registered as shoutrrrGenerateURL.
//
// Parameters:
//   - js.Value: unused, the JavaScript this value.
//   - args: the service name, then the config values as a JSON object.
//
// Returns:
//   - any: a JSON object with the URL in its url field, or an errorResult when
//     an argument is missing or a value is rejected, as a JSON string.
func generateURL(_ js.Value, args []js.Value) any {
	if len(args) < pairArgs {
		return marshalErrorStr("missing arguments: expected service name and config JSON")
	}

	return generateURLString(args[0].String(), args[1].String())
}

// validateURL checks that a Shoutrrr URL reads into a service.
//
// It is registered as shoutrrrValidateURL.
//
// Parameters:
//   - js.Value: unused, the JavaScript this value.
//   - args: the Shoutrrr URL.
//
// Returns:
//   - any: {"valid":true}, or an errorResult when the URL is missing or is
//     rejected, as a JSON string.
func validateURL(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalErrorStr("missing URL argument")
	}

	return validateURLString(args[0].String())
}

// send returns a JavaScript Promise for sending a message to a service URL.
//
// It is registered as shoutrrrSend. The send runs in a goroutine, so it does
// not block the JavaScript event loop.
//
// Parameters:
//   - js.Value: unused, the JavaScript this value.
//   - args: the Shoutrrr URL, then the message.
//
// Returns:
//   - any: a Promise that resolves with {"success":true} and rejects with an
//     errorResult as a JSON string, including when an argument is missing or
//     the send panics.
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
				// sendString returned text that is not JSON, so the rejection says so.
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
