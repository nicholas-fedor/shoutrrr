# Home Assistant End-to-End Tests

This directory contains end-to-end tests for the Home Assistant notification service.
The tests send real notifications to a local Home Assistant container and verify persistent notifications through the websocket `persistent_notification/get` command.

## Overview

`setup.sh` starts Home Assistant, completes onboarding, and writes `SHOUTRRR_HOMEASSISTANT_URL` to `.env`.

## Test Coverage

- Persistent notification create
- Title
- Notification ID replace
- HTTP (`disabletls=yes`)
- Unauthorized token

## Setup Requirements

- Go 1.27+
- Docker and Docker Compose
- curl
- python3
- Linux OS

### Quick Start

```bash
cd testing/e2e/homeassistant
./setup.sh setup-all
go test -v .
```

To stop the server:

```bash
./setup.sh stop-server
```

## Environment

`setup.sh` writes `.env`:

```bash
SHOUTRRR_HOMEASSISTANT_URL=homeassistant://TOKEN@localhost:8123/?disabletls=yes
```

The access token is created during onboarding. Run `setup-all` again if the token has expired.
