# SMTP End-to-End Tests

These tests send real email through Shoutrrr's SMTP service to local [Mailpit](https://mailpit.axllent.org/) containers.
Each test reads the delivered message back through Mailpit's API and checks it.

## Setup

```bash
./testing/e2e/smtp/setup.sh
```

That script generates self-signed TLS certificates if needed and starts Mailpit.
It then waits until both instances answer.

Mailpit serves STARTTLS or implicit TLS on a port, not both, so the stack runs two instances.

| Instance | SMTP port | API and web UI | TLS | Authentication |
| --- | --- | --- | --- | --- |
| `mailpit` | `1587` | `http://localhost:8025` | Optional STARTTLS | Requires `shoutrrr` / `testing`, PLAIN or LOGIN, also over plaintext |
| `mailpit-tls` | `1465` | `http://localhost:8026` | Implicit TLS | Accepts any credentials or none |

Both instances use the self-signed certificate, so the tests set `skiptlsverify=yes`.

## Run

```bash
go test -v ./testing/e2e/smtp/
```

The tests need no environment variables.
They skip when a Mailpit instance is not reachable.

## Coverage

- STARTTLS with PLAIN and with LOGIN authentication
- Plaintext with PLAIN authentication
- Implicit TLS without authentication
- Rejection of a wrong password (reply code 535) and of a send without credentials (reply code 530)
- The subject from the `title` param
- The sender name
- An HTML body with a plain text part
- A plus-tagged recipient and a second recipient, each of which receives its own message

## Cleanup

```bash
./testing/e2e/smtp/setup.sh stop-server
```
