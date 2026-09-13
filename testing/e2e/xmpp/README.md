# XMPP End-to-End Tests

These tests send real XMPP chat and MUC messages against a local ejabberd container.

## Setup

```bash
./testing/e2e/xmpp/setup.sh
```

That script generates self-signed TLS certificates if needed, starts ejabberd, and prints the environment variables required by the tests.

Ports:

- `5222` STARTTLS (use `skiptlsverify=yes` with the self-signed cert)
- `5223` implicit TLS
- `5224` plaintext (`disabletls=yes`; STARTTLS is not advertised)

## Accounts

Static credentials are registered on first container create via `CTL_ON_CREATE`:

- `sender@localhost` / `senderpass`
- `recipient@localhost` / `recipientpass`
- Room `alerts@conference.localhost`
- Room `secret@conference.localhost` with password `roompass`

## Run

```bash
export SHOUTRRR_XMPP_URL='xmpp://sender:senderpass@localhost:5222/?to=recipient@localhost&skiptlsverify=yes'
export SHOUTRRR_XMPP_TLS_URL='xmpps://sender:senderpass@localhost:5223/?to=recipient@localhost&skiptlsverify=yes'
export SHOUTRRR_XMPP_PLAIN_URL='xmpp://sender:senderpass@localhost:5224/?to=recipient@localhost&disabletls=yes'
export SHOUTRRR_XMPP_MUC_URL='xmpp://sender:senderpass@localhost:5222/?rooms=alerts@conference.localhost&skiptlsverify=yes'
export SHOUTRRR_XMPP_MUC_PASSWORD_URL='xmpp://sender:senderpass@localhost:5222/?rooms=secret@conference.localhost&roompassword=roompass&skiptlsverify=yes'
go test -v ./testing/e2e/xmpp/
```

Tests skip when the corresponding environment variable is unset.
