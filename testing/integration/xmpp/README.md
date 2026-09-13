# XMPP Integration Tests

These tests exercise the XMPP service through its public API with a mocked
session. They do not open a network connection to an XMPP server.

## Coverage

### Configuration

- URL parsing for `xmpp://` and `xmpps://`
- Default ports (5222 / 5223)
- Full auth JID vs dial host
- `disabletls`, `skiptlsverify`, `nick`, `roompassword`
- Invalid URLs (`disabletls` on xmpps, missing targets/password, invalid JIDs)
- `GetURL` round-trip

### Sending

- 1:1 chat
- MUC with default nick
- Combined chat and MUC with title prepend
- Multiple chat recipients
- Params override for nick and title

### Errors

- Dial failures
- Session factory failures
- Chat and MUC send failures
- Unknown params
- Params that clear all targets

### Edge cases

- Empty body
- Title-only
- Unicode
- Long bodies
- Newlines, tabs, and JSON

## Run

```bash
go test ./testing/integration/xmpp/ -v
```
