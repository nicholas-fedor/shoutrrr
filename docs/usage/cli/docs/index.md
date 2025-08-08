# Docs

## Overview

The `docs` command prints documentation for the respective service.

This is primarily used to generate service-specific information for the Shoutrrr website.

## Usage

```bash title="Docs Command Syntax"
shoutrrr docs [FLAG] <SERVICE>
```

### Flags

| Flag                 | Description                       |
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
Title      string
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

*  __Token__ (**Required**)
  URL part: <code class="service-url">discord://<strong>token</strong>@webhookid/</code>
*  __WebhookID__ (**Required**)
  URL part: <code class="service-url">discord://token@<strong>webhookid</strong>/</code>
### Query/Param Props

Props can be either supplied using the params argument, or through the URL using
`?key=value&key=value` etc.

*  __Avatar__ - Override the webhook default avatar with specified URL
  Default: *empty*
  Aliases: `avatarurl`

*  __Color__ - The color of the left border for plain messages
  Default: `0x50D9ff`

*  __ColorDebug__ - The color of the left border for debug messages
  Default: `0x7b00ab`

*  __ColorError__ - The color of the left border for error messages
  Default: `0xd60510`

*  __ColorInfo__ - The color of the left border for info messages
  Default: `0x2488ff`

*  __ColorWarn__ - The color of the left border for warning messages
  Default: `0xffc441`

*  __JSON__ - Whether to send the whole message as the JSON payload instead of using it as the 'content' field
  Default: ❌ `No`

*  __SplitLines__ - Whether to send each line as a separate embedded item
  Default: ✔ `Yes`

*  __ThreadID__ - The thread ID to send the message to
  Default: *empty*

*  __Title__
  Default: *empty*

*  __Username__ - Override the webhook default username
  Default: *empty*
```
