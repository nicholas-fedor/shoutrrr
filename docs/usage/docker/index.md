# Using Shoutrrr as a Docker Container

## Overview

The Shoutrrr Docker image provides a lightweight containerized version of the Shoutrrr CLI, built on Alpine Linux for minimal size and broad compatibility. It supports all architectures (amd64, arm64, arm/v6, i386, riscv64) and is available on Docker Hub (`nickfedor/shoutrrr`) and GHCR (`ghcr.io/nicholas-fedor/shoutrrr`). Tags include `latest` (stable production), versioned tags (e.g., `v0.8.0`), and `latest-dev` (development snapshots).

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

| Tag Examples          | Description                                      |
|-----------------------|--------------------------------------------------|
| `latest`              | Latest stable release.                           |
| `vX.Y.Z`              | Specific version (e.g., `v0.8.0`).               |
| `latest-dev`          | Latest development snapshot.                     |
| `amd64-latest`        | Platform-specific (e.g., amd64, arm64v8).        |

!!! Note
    The image includes CA certificates and timezone data. No volumes are required by default, but mount if needed for custom configs or stdin input. Environment variables can override flags (e.g., `SHOUTRRR_URL` for `--url`).

### Environment Variables

- Use uppercase flag names prefixed with `SHOUTRRR_` (e.g., `SHOUTRRR_MESSAGE` for `--message`).

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

### Verify a URL with Verbose Output

!!! Example
    ```bash title="Verify Slack URL"
    docker run --rm nickfedor/shoutrrr:latest verify --url "slack://token-a/token-b/token-c"
    ```

    ```text title="Expected Output"
    URL valid
    ```

### Send from Stdin with Environment Variables

!!! Example
    ```bash title="Send with Env Vars and Stdin"
    echo "Message from stdin" | docker run --rm -i -e SHOUTRRR_URL="slack://token-a/token-b/token-c" -e SHOUTRRR_MESSAGE="-" nickfedor/shoutrrr:latest send
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
    shoutrrr version latest
    ```
<!-- markdownlint-restore -->

## Notes

- **Multi-Architecture**: Use platform-specific tags (e.g., `arm64v8-latest`) or let Docker select automatically with `latest`.
- **Timeouts**: Inherits Shoutrrr's 10-second send timeout.
- **Volumes**: Mount `/etc/ssl/certs` if custom CA certs are needed, or `/input` for file-based messages.
- **Updates**: Pull latest images regularly. For production, pin to versioned tags.
- **Debugging**: Add `-v` for verbose output in commands.
