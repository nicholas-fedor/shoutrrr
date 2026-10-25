# Bark

Upstream docs: <https://github.com/Finb/Bark>

## URL Format

--8<-- "docs/services/push/bark/config.md"

## TLS

Bark sends over HTTPS by default.
Set `disabletls=yes` to send over HTTP instead, which leaves the connection unencrypted.
The first send without TLS logs a warning.

!!! warning "Deprecated"
    The `scheme` parameter is deprecated in favor of `disabletls`.
    It still works and logs a warning the first time it is used.
    A `scheme` of `http` turns TLS off, and any value other than `http` or `https` is rejected.
    In Go, the `Config.Scheme` field is deprecated in favor of `Config.DisableTLS`.
