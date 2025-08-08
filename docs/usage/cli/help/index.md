# Help

## Overview

The `help` command provides general information about available commands for the Shoutrrr CLI.

## Usage

```bash title="Help Command Syntax"
shoutrrr help
```

| Flag            | Description                                               |
|-----------------|-----------------------------------------------------------|
| `-h, --help`    | Displays help for the Shoutrrr CLI or a specific command. |
| `-v, --version` | Displays the Shoutrrr version information.                |

## Example

<!-- markdownlint-disable -->
!!! Example
    ```bash title="General Help Command"
    shoutrrr help
    ```

    ```text title="Expected Output"
    Shoutrrr CLI
    Usage:
      shoutrrr [command]
    Available Commands:
      completion Generate the autocompletion script for the specified shell
      docs Print documentation for services
      generate Generates a notification service URL from user input
      help Help about any command
      send Send a notification using a service url
      verify Verify the validity of a notification service URL
    Flags:
      -h, --help help for shoutrrr
      -v, --version version for shoutrrr
    Use "shoutrrr [command] --help" for more information about a command.
    ```
<!-- markdownlint-restore -->
