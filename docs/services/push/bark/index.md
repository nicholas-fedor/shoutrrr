# Bark

Upstream docs: <https://github.com/Finb/Bark>

## URL Format

--8<-- "docs/services/push/bark/config.md"

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
