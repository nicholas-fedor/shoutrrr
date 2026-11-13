# Bark

Upstream docs: <https://github.com/Finb/Bark>

## URL Format

!!! info ""
    bark://:__`DEVICE_KEY`__@__`HOST`__[:__`PORT`__][/__`PATH`__]

--8<-- "docs/services/push/bark/config.md"

## Badge

The `badge` parameter sets the number displayed on the app icon.
Shoutrrr sends it only when it is positive.
Bark clears the app's notifications from Notification Center when it receives a badge of 0, so a `badge` of 0 sends no badge and leaves earlier notifications in place.
A negative badge is rejected when the URL is read and when it is sent.

## TLS

Bark sends over HTTPS by default.
Set `disabletls=yes` to send over HTTP instead, which leaves the connection unencrypted.
The first send without TLS logs a warning.

A self-hosted bark-server can serve HTTPS with its own certificate, given through its `--cert` and `--key` options.
Set `skiptlsverify=yes` when that certificate cannot be verified, such as a self-signed one.
The connection stays encrypted, but Shoutrrr does not check which server it talks to.
The first send that skips verification logs a warning.

Both options can also be passed as send params, which apply to that send only.
A custom HTTP client set through `SetHTTPClient` or the `HTTPClient` field keeps its own TLS settings, so `skiptlsverify` does not change it.

!!! warning "Deprecated"
    The `scheme` parameter is deprecated in favor of `disabletls`.
    It still works and logs a warning the first time it is used.
    A `scheme` of `http` turns TLS off, and any value other than `http` or `https` is rejected.
    In Go, the `Config.Scheme` field is deprecated in favor of `Config.DisableTLS`.

!!! warning "Deprecated"
    The `category` parameter is deprecated and has no replacement.
    Bark sets the category of each notification itself, so Shoutrrr does not send it.
    A URL or send params that set it still work, and the first send with one logs a warning.
    A config URL that Shoutrrr writes leaves it out.
    In Go, the `Config.Category` and `PushPayload.Category` fields are deprecated too.
