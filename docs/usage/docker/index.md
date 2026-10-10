# Using Shoutrrr as a Docker Container

## Overview

The Shoutrrr Docker image provides a lightweight containerized version of the Shoutrrr CLI. It supports all architectures (amd64, arm64, arm/v6, i386, riscv64) and is available on Docker Hub (`nickfedor/shoutrrr`) and GHCR (`ghcr.io/nicholas-fedor/shoutrrr`). The image is built from `scratch` and contains only the static binary, CA certificates, and timezone data. Tags include `latest` (stable production), versioned tags (e.g., `0.21.2`), and `nightly` (rolling release).

## Usage

=== "Docker Hub"

    ```bash title="Pull Command Syntax"
    docker pull nickfedor/shoutrrr:latest
    ```

=== "GHCR"

    ```bash title="Pull Command Syntax"
    docker pull ghcr.io/nicholas-fedor/shoutrrr:latest
    ```

Run Shoutrrr CLI commands inside the container using `docker run`.

The entrypoint is `/shoutrrr`, so commands like `send`, `generate`, `verify` work directly.

| Tag Examples   | Description                               |
|----------------|-------------------------------------------|
| `latest`       | Latest stable release.                    |
| `X.Y.Z`        | Specific version (e.g., `0.21.2`).        |
| `X.Y`, `X`     | Latest release of a minor or major.       |
| `nightly`      | Latest rolling release.                   |
| `amd64-latest` | Platform-specific (e.g., amd64, arm64v8). |

!!! Note
    The image includes CA certificates and timezone data. No volumes are required by default, but mount if needed for custom configs or stdin input. Environment variables can stand in for flags (e.g., `SHOUTRRR_URL` for `--url`).

### Environment Variables

- Use uppercase flag names prefixed with `SHOUTRRR_`, with dashes replaced by underscores (e.g., `SHOUTRRR_MESSAGE` for
  `--message`). A flag given on the command line takes precedence. See the
  [CLI environment variables](../cli/index.md#environment-variables) for the full list.

## Examples

<!-- markdownlint-disable -->
### Send a Notification

!!! Example
    ```bash title="Send to Discord"
    docker run --rm nickfedor/shoutrrr:latest send --url "discord://abc123@123456789" --message "Hello, Docker!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Generate a Service URL

!!! Example
    ```bash title="Generate Discord URL"
    docker run --rm -it nickfedor/shoutrrr:latest generate discord
    ```

    ```text title="Expected Prompt Inputs"
    Generating URL for discord using basic generator

    Token: abc123
    WebhookID: 123456789
    ```

    ```text title="Expected Output"
    URL: discord://abc123@123456789
    ```

### Verify a URL

!!! Example
    ```bash title="Verify Slack URL"
    docker run --rm nickfedor/shoutrrr:latest verify --url "slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook"
    ```

    ```text title="Expected Output (abridged)"
    BotName                                Bot name                                                     <Aliases: username>
    Channel  webhook                       Channel to send messages to in Cxxxxxxxxxx format            <URL: Host> <Required>
    ...
    ```

### Send from Stdin with Environment Variables

!!! Example
    ```bash title="Send with Env Vars and Stdin"
    echo "Message from stdin" | docker run --rm -i -e SHOUTRRR_URL="slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook" -e SHOUTRRR_MESSAGE="-" nickfedor/shoutrrr:latest send
    ```

    ```text title="Expected Output"
    Reading from STDIN...
    Read 20 byte(s)
    Notification sent
    ```

### Multi-Architecture Pull and Run

!!! Example
    ```bash title="Pull and Run on ARM64"
    docker pull nickfedor/shoutrrr:arm64v8-latest
    docker run --rm nickfedor/shoutrrr:arm64v8-latest --version
    ```

    ```text title="Expected Output"
    shoutrrr version 0.21.2 (Built on 2026-09-30 from Git SHA 1a2b3c4)
    ```
<!-- markdownlint-restore -->

## Notes

- **Multi-Architecture**: Use platform-specific tags (e.g., `arm64v8-latest`) or let Docker select automatically with `latest`.
- **Timeouts**: The default is 10 seconds per service. A longer service timeout, such as SMTP `timeout`, extends that service. The CLI does not set a fixed sender timeout.
- **Volumes**: Mount `/etc/ssl/certs` if custom CA certs are needed. Pass message text through stdin with `-i` and `--message -`.
- **Updates**: Pull latest images regularly. For production, pin to versioned tags.
- **Debugging**: Add `-v` for verbose output in commands.
