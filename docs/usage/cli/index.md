# Overview

Shoutrrr can be used via the CLI to both generate and verify service notification URL's, in addition to sending notifications.

```bash title="Shoutrrr run without arguments"
shoutrrr
```

```text title="Expected Output"
Shoutrrr CLI

Usage:
  shoutrrr [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  docs        Print documentation for services
  generate    Generates a notification service URL from user input
  help        Help about any command
  send        Send a notification using a service url
  verify      Verify the validity of a notification service URL

Flags:
  -h, --help      help for shoutrrr
  -v, --version   version for shoutrrr

Use "shoutrrr [command] --help" for more information about a command.
```

## Environment Variables

The `send`, `verify` and `generate` commands read any flag that is not given on the command line from an environment
variable. The variable name is `SHOUTRRR_` followed by the flag name in upper case, with dashes replaced by underscores.

| Variable                  | Flag               | Commands                    |
|---------------------------|--------------------|-----------------------------|
| `SHOUTRRR_URL`            | `--url`            | `send`, `verify`            |
| `SHOUTRRR_MESSAGE`        | `--message`        | `send`                      |
| `SHOUTRRR_TITLE`          | `--title`          | `send`                      |
| `SHOUTRRR_VERBOSE`        | `--verbose`        | `send`                      |
| `SHOUTRRR_SERVICE`        | `--service`        | `generate`                  |
| `SHOUTRRR_GENERATOR`      | `--generator`      | `generate`                  |
| `SHOUTRRR_PROPERTY`       | `--property`       | `generate`                  |
| `SHOUTRRR_SHOW_SENSITIVE` | `--show-sensitive` | `generate`                  |

- A flag or positional argument takes precedence over its environment variable, and an empty variable counts as unset.
- Positional arguments fill only flags that were not given on the command line.
- A flag that accepts several values, such as `--url` or `--property`, takes a single value from its variable.
- When the URL comes from `SHOUTRRR_URL` and no message is given, `send` reads the message from stdin.
