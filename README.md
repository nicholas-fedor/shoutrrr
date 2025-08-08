<!-- markdownlint-disable -->
<div align="center">

<a href="https://github.com/nicholas-fedor/shoutrrr">
    <img src="https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/refs/heads/main/docs/assets/media/shoutrrr-logotype.svg" width="450" />
</a>

# Shoutrrr

A notification library for gophers and their furry friends.<br />
Heavily inspired by <a href="https://github.com/caronc/apprise">caronc/apprise</a>.

![github actions workflow status](https://github.com/nicholas-fedor/shoutrrr/workflows/Main%20Workflow/badge.svg)
[![codecov](https://codecov.io/gh/nicholas-fedor/shoutrrr/branch/main/graph/badge.svg)](https://codecov.io/gh/nicholas-fedor/shoutrrr)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/47eed72de79448e2a6e297d770355544)](https://www.codacy.com/gh/nicholas-fedor/shoutrrr/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=nicholas-fedor/shoutrrr&amp;utm_campaign=Badge_Grade)
[![report card](https://goreportcard.com/badge/github.com/nicholas-fedor/shoutrrr)](https://goreportcard.com/badge/github.com/nicholas-fedor/shoutrrr)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/nicholas-fedor/shoutrrr)
[![github code size in bytes](https://img.shields.io/github/languages/code-size/nicholas-fedor/shoutrrr.svg?style=flat-square)](https://github.com/nicholas-fedor/shoutrrr)
[![license](https://img.shields.io/github/license/nicholas-fedor/shoutrrr.svg?style=flat-square)](https://github.com/nicholas-fedor/shoutrrr/blob/main/LICENSE)
[![Pulls from DockerHub](https://img.shields.io/docker/pulls/nickfedor/shoutrrr.svg)](https://hub.docker.com/r/nickfedor/shoutrrr)
[![godoc](https://godoc.org/github.com/nicholas-fedor/shoutrrr?status.svg)](https://godoc.org/github.com/nicholas-fedor/shoutrrr) <!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-16-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
</div>
<!-- markdownlint-restore -->

## Table of Contents

- [Full Documentation](#full-documentation)
- [Installation](#installation)
  - [From Source](#from-source)
  - [Binaries](#binaries)
  - [Container Images](#container-images)
  - [Go Package](#go-package)
  - [GitHub Action](#github-action)
- [Usage](#usage)
  - [CLI](#cli)
  - [Go Package Usage](#go-package-usage)
  - [Docker](#docker)
  - [GitHub Action Usage](#github-action-usage)
- [Supported Services](#supported-services)
- [Contributors](#contributors-)
- [Related Projects](#related-projects)

## Full Documentation

Visit the project's [GitHub Page](https://nicholas-fedor.github.io/shoutrrr) for full documentation.

## Installation

### From Source

```bash
go install github.com/nicholas-fedor/shoutrrr/shoutrrr@latest
```

### Binaries

Install the latest release binary to `$HOME/go/bin` (ensure it's in your `PATH`).

- **Windows (amd64):**

  ```powershell
  New-Item -ItemType Directory -Path $HOME\go\bin -Force | Out-Null; iwr (iwr https://api.github.com/repos/nicholas-fedor/shoutrrr/releases/latest | ConvertFrom-Json).assets.where({$_.name -like "*windows_amd64*.zip"}).browser_download_url -OutFile shoutrrr.zip; Add-Type -AssemblyName System.IO.Compression.FileSystem; ($z=[System.IO.Compression.ZipFile]::OpenRead("$PWD\shoutrrr.zip")).Entries | ? {$_.Name -eq 'shoutrrr.exe'} | % {[System.IO.Compression.ZipFileExtensions]::ExtractToFile($_, "$HOME\go\bin\$($_.Name)", $true)}; $z.Dispose(); rm shoutrrr.zip; if (Test-Path "$HOME\go\bin\shoutrrr.exe") { Write-Host "Successfully installed shoutrrr.exe to $HOME\go\bin" } else { Write-Host "Failed to install shoutrrr.exe" }
  ```

- **Linux (amd64):**

  ```bash
  mkdir -p $HOME/go/bin && curl -L $(curl -s https://api.github.com/repos/nicholas-fedor/shoutrrr/releases/latest | grep -o 'https://[^"]*linux_amd64[^"]*\.tar\.gz') | tar -xz --strip-components=1 -C $HOME/go/bin shoutrrr
  ```

- **macOS (amd64):**

  ```bash
  mkdir -p $HOME/go/bin && curl -L $(curl -s https://api.github.com/repos/nicholas-fedor/shoutrrr/releases/latest | grep -o 'https://[^"]*darwin_amd64[^"]*\.tar\.gz') | tar -xz --strip-components=1 -C $HOME/go/bin shoutrrr
  ```

> [!Note]
> Visit the [releases page](https://github.com/nicholas-fedor/shoutrrr/releases) for other architectures (e.g., arm, arm64, i386, riscv64).

### Container Images

- **[Docker Hub](https://hub.docker.com/r/nickfedor/shoutrrr):**

  ```bash
  docker pull nickfedor/shoutrrr:latest
  ```

- **[GHCR](https://github.com/users/nicholas-fedor/packages/container/package/shoutrrr):**

  ```bash
  docker pull ghcr.io/nicholas-fedor/shoutrrr:latest
  ```

> [!Note]
> Tags: `latest` (stable), `vX.Y.Z` (specific version), `latest-dev` (development), platform-specific (e.g., `amd64-latest`).

### Go Package

```bash
go get github.com/nicholas-fedor/shoutrrr@latest
```

### GitHub Action

```yaml
- name: Shoutrrr
  uses: nicholas-fedor/shoutrrr-action@v1
  with:
    url: ${{ secrets.SHOUTRRR_URL }}
    title: Deployed ${{ github.sha }}
    message: See changes at ${{ github.event.compare }}.
```

## Usage

### CLI

```bash
shoutrrr send --url "slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook" --message "Hello, Slack!"
```

### Go Package Usage

```go
import "github.com/nicholas-fedor/shoutrrr"

errs := shoutrrr.Send("slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook", "Hello, Slack!")
if len(errs) > 0 {
    // Handle errors
}
```

### Docker

```bash
docker run --rm nickfedor/shoutrrr:latest send --url "slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook" --message "Hello, Slack!"
```

### GitHub Action Usage

See installation example [above](#github-action).

### Use as a Package

#### Option 1 - Using the direct send command

```go
url := "slack://token-a/token-b/token-c"
err := shoutrrr.Send(url, "Hello world (or slack channel) !")
```

#### Option 2 - Using a sender

##### Single URL

```go
url := "slack://token-a/token-b/token-c"
sender, err := shoutrrr.CreateSender(url)
sender.Send("Hello world (or slack channel) !", map[string]string { /* ... */ })
```

##### Multiple URLs

```go
urls := []string {
  "slack://token-a/token-b/token-c"
  "discord://token@channel"
}
sender, err := shoutrrr.CreateSender(urls...)
sender.Send("Hello world (or slack channel) !", map[string]string { /* ... */ })
```

### Use Through the CLI

```bash
shoutrrr send [OPTIONS] <URL> <Message [...]>
```

### Use as a GitHub Action

You can also use Shoutrrr in a GitHub Actions workflow.

```yaml
name: Deploy
on:
  push:
    branches:
      - main

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - [Your other workflow steps]

      - name: Shoutrrr
        uses: nicholas-fedor/shoutrrr-action@v0.0.11
        with:
          url: ${{ secrets.SHOUTRRR_URL }}
          title: Deployed ${{ github.sha }}
          message: See changes at ${{ github.event.compare }}.
```

## Supported Services

| Service      | Description                          |
|--------------|--------------------------------------|
| Bark         | iOS push notifications               |
| Discord      | Discord webhooks                     |
| Generic      | Custom HTTP webhooks                 |
| Google Chat  | Google Chat webhooks                 |
| Gotify       | Gotify push notifications            |
| IFTTT        | IFTTT webhooks                       |
| Join         | Join push notifications              |
| Lark         | Lark (Feishu) webhooks               |
| Logger       | Local logging (for testing)          |
| Matrix       | Matrix rooms                         |
| Mattermost   | Mattermost webhooks                  |
| Ntfy         | Ntfy push notifications              |
| Opsgenie     | Opsgenie alerts                      |
| Pushbullet   | Pushbullet push notifications        |
| Pushover     | Pushover push notifications          |
| Rocket.Chat  | Rocket.Chat webhooks                 |
| Slack        | Slack webhooks or Bot API            |
| SMTP         | Email notifications                  |
| Teams        | Microsoft Teams webhooks             |
| Telegram     | Telegram bots                        |
| Zulip        | Zulip chat                           |
| XMPP         | XMPP messages (if enabled)           |

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- markdownlint-disable -->
<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/nicholas-fedor"><img src="https://avatars2.githubusercontent.com/u/71477161?v=4?s=100" width="100px;" alt="Nicholas Fedor"/><br /><sub><b>Nicholas Fedor</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=nicholas-fedor" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=nicholas-fedor" title="Documentation">📖</a> <a href="#maintenance-nicholas-fedor" title="Maintenance">🚧</a> <a href="https://github.com/nicholas-fedor/shoutrrr/pulls?q=is%3Apr+reviewed-by%3Anicholas-fedor" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/amirschnell"><img src="https://avatars3.githubusercontent.com/u/9380508?v=4?s=100" width="100px;" alt="Amir Schnell"/><br /><sub><b>Amir Schnell</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=amirschnell" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://piksel.se"><img src="https://avatars2.githubusercontent.com/u/807383?v=4?s=100" width="100px;" alt="nils måsén"/><br /><sub><b>nils måsén</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=piksel" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=piksel" title="Documentation">📖</a> <a href="#maintenance-piksel" title="Maintenance">🚧</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/lukapeschke"><img src="https://avatars1.githubusercontent.com/u/17085536?v=4?s=100" width="100px;" alt="Luka Peschke"/><br /><sub><b>Luka Peschke</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=lukapeschke" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=lukapeschke" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/MrLuje"><img src="https://avatars0.githubusercontent.com/u/632075?v=4?s=100" width="100px;" alt="MrLuje"/><br /><sub><b>MrLuje</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=MrLuje" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=MrLuje" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://simme.dev"><img src="https://avatars0.githubusercontent.com/u/1596025?v=4?s=100" width="100px;" alt="Simon Aronsson"/><br /><sub><b>Simon Aronsson</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=simskij" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=simskij" title="Documentation">📖</a> <a href="#maintenance-simskij" title="Maintenance">🚧</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://arnested.dk"><img src="https://avatars2.githubusercontent.com/u/190005?v=4?s=100" width="100px;" alt="Arne Jørgensen"/><br /><sub><b>Arne Jørgensen</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=arnested" title="Documentation">📖</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=arnested" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/atighineanu"><img src="https://avatars1.githubusercontent.com/u/27206712?v=4?s=100" width="100px;" alt="Alexei Tighineanu"/><br /><sub><b>Alexei Tighineanu</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=atighineanu" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ellisab"><img src="https://avatars2.githubusercontent.com/u/1402047?v=4?s=100" width="100px;" alt="Alexandru Bonini"/><br /><sub><b>Alexandru Bonini</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=ellisab" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://senan.xyz"><img src="https://avatars0.githubusercontent.com/u/6832539?v=4?s=100" width="100px;" alt="Senan Kelly"/><br /><sub><b>Senan Kelly</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=sentriz" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/JonasPf"><img src="https://avatars.githubusercontent.com/u/2216775?v=4?s=100" width="100px;" alt="JonasPf"/><br /><sub><b>JonasPf</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=JonasPf" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/claycooper"><img src="https://avatars.githubusercontent.com/u/3612906?v=4?s=100" width="100px;" alt="claycooper"/><br /><sub><b>claycooper</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=claycooper" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://ko-fi.com/disyer"><img src="https://avatars.githubusercontent.com/u/16326697?v=4?s=100" width="100px;" alt="Derzsi Dániel"/><br /><sub><b>Derzsi Dániel</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=darktohka" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://josephkav.io"><img src="https://avatars.githubusercontent.com/u/4267227?v=4?s=100" width="100px;" alt="Joseph Kavanagh"/><br /><sub><b>Joseph Kavanagh</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=JosephKav" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/issues?q=author%3AJosephKav" title="Bug reports">🐛</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://ring0.lol"><img src="https://avatars.githubusercontent.com/u/1893909?v=4?s=100" width="100px;" alt="Justin Steven"/><br /><sub><b>Justin Steven</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/issues?q=author%3Ajustinsteven" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/serverleader"><img src="https://avatars.githubusercontent.com/u/34089?v=4?s=100" width="100px;" alt="Carlos Savcic"/><br /><sub><b>Carlos Savcic</b></sub></a><br /><a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=serverleader" title="Code">💻</a> <a href="https://github.com/nicholas-fedor/shoutrrr/commits?author=serverleader" title="Documentation">📖</a></td>
    </tr>
  </tbody>
</table>
<!-- ALL-CONTRIBUTORS-LIST:END -->
<!-- markdownlint-restore -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!

## Related Project(s)

- [Watchtower](https://github.com/nicholas-fedor/watchtower) - Automate Docker container image updates
- [Shoutrrr GitHub Action](https://github.com/nicholas-fedor/shoutrrr-action) - Notifications using Shoutrrr in GitHub Actions
