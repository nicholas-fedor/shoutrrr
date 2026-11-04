# Deprecations

Shoutrrr keeps deprecated options and APIs working, so URLs and code that use them keep working after an update.
Each one is listed here with its replacement.
Removal is not planned.

New code should use the replacements, and existing code can move over at any time.

## URL Settings

Deprecated query keys and URL schemes are still accepted.
A deprecated query key also works in send params.
When a URL or the send params hold both a deprecated query key and its replacement, the replacement wins.

| Setting | Service | Replacement | Deprecated in |
| --- | --- | --- | --- |
| `date` query key | Gotify | None. Gotify stamps each message with the server time, so the key has no effect. | v0.22.1 |
| `hangouts://` scheme | Google Chat | `googlechat://` | Inherited from containrrr/shoutrrr |
| `insecureskipverify` query key | Gotify | `skiptlsverify` | v0.23.0 |
| `disabletlsverification` query key | ntfy | `skiptlsverify` | v0.23.0 |
| `scheme` query key | ntfy | `disabletls`. A `scheme` of `http` turns TLS off. | v0.23.0 |
| `scheme` query key | Bark | `disabletls`. A `scheme` of `http` turns TLS off. | v0.23.0 |
| `disabletlsverification` query key | MQTT | `skiptlsverify` | v0.23.0 |

## Go Package

Deprecated identifiers carry a `Deprecated:` doc comment, which editors and linters such as staticcheck (SA1019) flag.
Those marked with `//go:fix inline` are rewritten to their replacements by `go fix ./...`.

A deprecated config field that has a replacement still applies.
When a config sets both a deprecated field and its replacement, a replacement with a non-zero value wins.
A replacement set to its zero value, such as `false` or an empty string, counts as unset, so the deprecated field applies.
A deprecated client field on a service and `Service.SetHTTPClient` both set the client, and the one set last carries the sends.

| Identifier | Package | Replacement | Deprecated in |
| --- | --- | --- | --- |
| `CreateSender` | `shoutrrr` | `CreateSenderWithOptions` | v0.16.2 |
| `NewSender` | `shoutrrr` | `NewSenderWithOptions` | v0.16.2 |
| `New` | `pkg/router` | `NewWithOptions` | v0.16.2 |
| `DefaultClient` | `pkg/util/jsonclient` | `NewWithHTTPClient` with an explicit client | v0.16.2 |
| `NewClient` | `pkg/util/jsonclient` | `NewWithHTTPClient` with an explicit client | v0.16.2 |
| `Get` | `pkg/util/jsonclient` | The `Get` method of a client from `NewWithHTTPClient` | v0.16.2 |
| `Post` | `pkg/util/jsonclient` | The `Post` method of a client from `NewWithHTTPClient` | v0.16.2 |
| `Service.SendItems` | `pkg/services/chat/discord` | `Service.SendItemsContext` | v0.22.0 |
| `Service.SendItems` | `pkg/services/push/bark` | `Service.Send` with `types.ItemsToPlain` | v0.22.0 |
| `Service.SendWithContext` | `pkg/services/chat/matrix` | `Service.SendContext` | v0.22.0 |
| `Service.SendWithContext` | `pkg/services/chat/zulip` | `Service.SendContext` | v0.22.0 |
| `Service.SendWithContext` | `pkg/services/incident/pagerduty` | `Service.SendContext` | v0.22.0 |
| `MaximumNArgs` | `shoutrrr/cmd/generate` | None. `generate` takes any number of generator arguments. | v0.22.0 |
| `EventPayload.Details` | `pkg/services/incident/pagerduty` | `Payload.CustomDetails` | v0.22.1 |
| `EventPayload.Contexts` | `pkg/services/incident/pagerduty` | `EventPayload.Links` and `EventPayload.Images` | v0.22.1 |
| `Config.Date` | `pkg/services/push/gotify` | None. The date is not sent. | v0.22.1 |
| `MessageRequest.Date` | `pkg/services/push/gotify` | None. The date is not sent. | v0.22.1 |
| `Config.ValidateCredentials` | `pkg/services/push/mqtt` | None. MQTT v5 allows a password without a username. | v0.22.1 |
| `ErrPasswordWithoutUsername` | `pkg/services/push/mqtt` | None. MQTT v5 allows a password without a username. | v0.22.1 |
| `HTTPClientManager` | `pkg/services/push/gotify` | `Service.SetHTTPClient` | v0.23.0 |
| `DefaultHTTPClientManager` | `pkg/services/push/gotify` | `Service.SetHTTPClient` | v0.23.0 |
| `HTTPClient` | `pkg/services/chat/matrix` | `types.HTTPClient` | v0.23.0 |
| `DefaultHTTPClient` | `pkg/services/chat/matrix` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `HTTPClient` | `pkg/services/chat/teams` | `types.HTTPClient` | v0.23.0 |
| `Service.Client` | `pkg/services/chat/rocketchat` | `Service.SetHTTPClient` | v0.23.0 |
| `ErrInvalidURLScheme` | `pkg/services/chat/rocketchat` | None. No function returns it, and `disabletls` sends over HTTP. | v0.23.0 |
| `ErrMissingHost` | `pkg/services/chat/lark` | None. No function returns it, because a config without a host uses Lark's host. | v0.23.0 |
| `Service.Client` | `pkg/services/push/pushover` | `Service.SetHTTPClient` | v0.23.0 |
| `HTTPClient` | `pkg/services/chat/discord` | `types.HTTPClient` | v0.23.0 |
| `DefaultHTTPClient` | `pkg/services/chat/discord` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `NewDefaultHTTPClient` | `pkg/services/chat/discord` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `Service.HTTPClient` | `pkg/services/chat/discord` | `Service.SetHTTPClient` | v0.23.0 |
| `HTTPClient` | `pkg/services/chat/zulip` | `types.HTTPClient` | v0.23.0 |
| `DefaultHTTPClient` | `pkg/services/chat/zulip` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `NewDefaultHTTPClient` | `pkg/services/chat/zulip` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `Service.HTTPClient` | `pkg/services/chat/zulip` | `Service.SetHTTPClient` | v0.23.0 |
| `HTTPClient` | `pkg/services/sms/twilio` | `types.HTTPClient` | v0.23.0 |
| `DefaultHTTPClient` | `pkg/services/sms/twilio` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `Service.HTTPClient` | `pkg/services/sms/twilio` | `Service.SetHTTPClient` | v0.23.0 |
| `HTTPClient` | `pkg/services/push/bark` | `types.HTTPClient` | v0.23.0 |
| `DefaultHTTPClient` | `pkg/services/push/bark` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `NewDefaultHTTPClient` | `pkg/services/push/bark` | `Service.SetHTTPClient` with an `*http.Client` | v0.23.0 |
| `Service.HTTPClient` | `pkg/services/push/bark` | `Service.SetHTTPClient` | v0.23.0 |
| `Config.Scheme` | `pkg/services/push/bark` | `Config.DisableTLS` | v0.23.0 |
| `Config.InsecureSkipVerify` | `pkg/services/push/gotify` | `Config.SkipTLSVerify` | v0.23.0 |
| `HTTPTimeout` | `pkg/services/push/gotify` | None. A send is bounded by the budget that `Service.ServiceTimeout` reports. | v0.23.0 |
| `Config.DisableTLSVerification` | `pkg/services/push/ntfy` | `Config.SkipTLSVerify` | v0.23.0 |
| `Config.Scheme` | `pkg/services/push/ntfy` | `Config.DisableTLS` | v0.23.0 |
| `HTTPTimeout` | `pkg/services/push/ntfy` | None. A send is bounded by the budget that `Service.ServiceTimeout` reports. | v0.23.0 |
| `Config.DisableTLSVerification` | `pkg/services/push/mqtt` | `Config.SkipTLSVerify` | v0.23.0 |
| `Logger.Log` | `pkg/services/standard` | `Logger.Debug`, `Logger.Info`, or `Logger.Warn` | v0.23.0 |
| `Logger.Logf` | `pkg/services/standard` | `Logger.Debug`, `Logger.Info`, or `Logger.Warn` | v0.23.0 |
