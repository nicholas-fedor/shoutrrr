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
| `insecureskipverify` query key | Gotify | `skiptlsverify` | Unreleased |
| `disabletlsverification` query key | ntfy | `skiptlsverify` | Unreleased |
| `scheme` query key | ntfy | `disabletls`. A `scheme` of `http` turns TLS off. | Unreleased |
| `scheme` query key | Bark | `disabletls`. A `scheme` of `http` turns TLS off. | Unreleased |
| `disabletlsverification` query key | MQTT | `skiptlsverify` | Unreleased |

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
| `HTTPClientManager` | `pkg/services/push/gotify` | `Service.SetHTTPClient` | Unreleased |
| `DefaultHTTPClientManager` | `pkg/services/push/gotify` | `Service.SetHTTPClient` | Unreleased |
| `HTTPClient` | `pkg/services/chat/matrix` | `types.HTTPClient` | Unreleased |
| `DefaultHTTPClient` | `pkg/services/chat/matrix` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `HTTPClient` | `pkg/services/chat/teams` | `types.HTTPClient` | Unreleased |
| `HTTPClient` | `pkg/services/chat/discord` | `types.HTTPClient` | Unreleased |
| `DefaultHTTPClient` | `pkg/services/chat/discord` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `NewDefaultHTTPClient` | `pkg/services/chat/discord` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `Service.HTTPClient` | `pkg/services/chat/discord` | `Service.SetHTTPClient` | Unreleased |
| `HTTPClient` | `pkg/services/chat/zulip` | `types.HTTPClient` | Unreleased |
| `DefaultHTTPClient` | `pkg/services/chat/zulip` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `NewDefaultHTTPClient` | `pkg/services/chat/zulip` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `Service.HTTPClient` | `pkg/services/chat/zulip` | `Service.SetHTTPClient` | Unreleased |
| `HTTPClient` | `pkg/services/push/bark` | `types.HTTPClient` | Unreleased |
| `DefaultHTTPClient` | `pkg/services/push/bark` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `NewDefaultHTTPClient` | `pkg/services/push/bark` | `Service.SetHTTPClient` with an `*http.Client` | Unreleased |
| `Service.HTTPClient` | `pkg/services/push/bark` | `Service.SetHTTPClient` | Unreleased |
| `Config.Scheme` | `pkg/services/push/bark` | `Config.DisableTLS` | Unreleased |
| `Config.InsecureSkipVerify` | `pkg/services/push/gotify` | `Config.SkipTLSVerify` | Unreleased |
| `HTTPTimeout` | `pkg/services/push/gotify` | None. A send is bounded by the budget that `Service.ServiceTimeout` reports. | Unreleased |
| `Config.DisableTLSVerification` | `pkg/services/push/ntfy` | `Config.SkipTLSVerify` | Unreleased |
| `Config.Scheme` | `pkg/services/push/ntfy` | `Config.DisableTLS` | Unreleased |
| `HTTPTimeout` | `pkg/services/push/ntfy` | None. A send is bounded by the budget that `Service.ServiceTimeout` reports. | Unreleased |
| `Config.DisableTLSVerification` | `pkg/services/push/mqtt` | `Config.SkipTLSVerify` | Unreleased |
