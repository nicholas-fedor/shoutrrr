# Docs

## Overview

The `docs` command prints documentation for the respective service.

This is primarily used to generate service-specific information for the Shoutrrr website.

## Usage

```bash title="Docs Command Syntax"
shoutrrr docs [FLAG] <SERVICE>
```

### Flags

| Flag                  | Description                       |
|-----------------------|-----------------------------------|
| `-f, --format string` | Output format (default "console") |
| `-h, --help`          | Help for `docs` command           |

#### Output Formats

| Format     | Description                    |
|------------|--------------------------------|
| `console`  | Output to the terminal console |
| `markdown` | Output in Markdown format      |

## Examples

### Output Service Docs to Console

```bash title="Print Discord service docs to console"
shoutrrr docs discord
```

```bash title="Expected Result"
Avatar     string          Override the webhook default avatar with specified URL       <Aliases: avatarurl>
Color      uint            The color of the left border for plain messages              <Default: 0x50D9ff>
ColorDebug uint            The color of the left border for debug messages              <Default: 0x7b00ab>
ColorError uint            The color of the left border for error messages              <Default: 0xd60510>
ColorInfo  uint            The color of the left border for info messages               <Default: 0x2488ff>
ColorWarn  uint            The color of the left border for warning messages            <Default: 0xffc441>
JSON       bool            Whether to send the whole message as the JSON payload instead of using it as the 'content' field  <Default: No>
SplitLines bool            Whether to send each line as a separate embedded item        <Default: Yes>
ThreadID   string          The thread ID to send the message to
Title      string          The title of the first embedded item
Token      string                                                                       <URL: User> <Required>
Username   string          Override the webhook default username
WebhookID  string                                                                       <URL: Host> <Required>
```

### Output Markdown-formatted Service Docs

```bash title="Print Discord service docs in markdown format"
shoutrrr docs --format markdown discord
```

```markdown title="Expected Result"
### URL Fields

* __Token__ (__Required__)
<br>URL part: User
* __WebhookID__ (__Required__)
<br>URL part: Host

### Query/Param Props

Props can be either supplied using the params argument or through the URL using
`?key=value&key=value` etc.

* __avatar__ - Override the webhook default avatar with specified URL
  Default: *empty*
  Aliases: `avatarurl`

* __color__ - The color of the left border for plain messages
  Default: `0x50D9ff`

* __colordebug__ - The color of the left border for debug messages
  Default: `0x7b00ab`

* __colorerror__ - The color of the left border for error messages
  Default: `0xd60510`

* __colorinfo__ - The color of the left border for info messages
  Default: `0x2488ff`

* __colorwarn__ - The color of the left border for warning messages
  Default: `0xffc441`

* __json__ - Whether to send the whole message as the JSON payload instead of using it as the 'content' field
  Default: ❌ `No`

* __splitlines__ - Whether to send each line as a separate embedded item
  Default: ✔ `Yes`

* __thread_id__ - The thread ID to send the message to
  Default: *empty*

* __title__ - The title of the first embedded item
  Default: *empty*

* __username__ - Override the webhook default username
  Default: *empty*
```
