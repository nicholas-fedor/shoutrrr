# Signalgrid End-to-End Tests

This directory contains end-to-end tests for the Signalgrid push notification service.
The tests send real notifications through the Signalgrid Push API.

## Coverage

- **Basic Test** (`basic_test.go`): Sends a plain notification
- **Title Test** (`title_test.go`): Sends a notification with a `title` query parameter
- **Type and Critical Test** (`type_critical_test.go`): Sends `type=CRIT` and `critical=true` notifications

## Setup

1. Create a [Signalgrid account](https://web.signalgrid.co/)
2. Copy your [client key](https://docs.signalgrid.co/get-started/find-client-key/)
3. Copy a [channel token](https://docs.signalgrid.co/get-started/find-channel-token/)

### Environment variable

Create `testing/e2e/signalgrid/.env`:

```bash
SHOUTRRR_SIGNALGRID_URL=signalgrid://CLIENT_KEY@CHANNEL
```

Never commit real credentials. `.env` is gitignored.

Or set the variable inline:

```bash
SHOUTRRR_SIGNALGRID_URL='signalgrid://CLIENT_KEY@CHANNEL' \
  go test ./testing/e2e/signalgrid/ -v
```

Tests skip when `SHOUTRRR_SIGNALGRID_URL` is unset.
