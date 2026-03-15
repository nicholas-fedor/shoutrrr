# Contributing to Shoutrrr

Original Repository:
<https://github.com/containrrr/shoutrrr>

Fork Repository:
<https://github.com/nicholas-fedor/shoutrrr>

## Getting Started

### Cloning the Repository

```bash
git clone https://github.com/nicholas-fedor/shoutrrr.git
```

### Development Tools

- [Make](https://www.gnu.org/software/make/)

  - Windows

    ```powershell
    winget install --id GnuWin32.Make
    ```

  - Linux

      Ubuntu/Debian:

      ```bash
      sudo apt install make -y
      ```

      Fedora/RHEL:

      ```bash
      sudo dnf install make
      ```

      Arch:

      ```bash
      sudo pacman -S make
      ```

  - macOS

    ```bash
    brew install make
    ```

- [Go](https://go.dev/)

  Installation Documentation: <https://go.dev/doc/install>

- [Golangci-Lint](https://golangci-lint.run/)

  Installation Documentation: <https://golangci-lint.run/welcome/install/#local-installation>

- [GoReleaser](https://goreleaser.com/)

  Installation Documentation: <https://goreleaser.com/install/>

- [MkDocs](https://www.mkdocs.org/)

  Installation Documentation: <https://www.mkdocs.org/user-guide/installation/>

  1. Confirm that you have Python and Pip installed:

      ```bash
      python --version
      pip --version
      ```

  2. Install MkDocs:

      ```bash
      pip install mkdocs
      ```

  3. Install Shoutrrr's dependencies:

      ```bash
      pip install -r build/mkdocs/docs-requirements.txt
      ```

## Building and Testing

Shoutrrr is a Go library and is built with Go commands.
The following commands assume that you are at the root level of your repo.

```bash
./scripts/build.sh                     # compiles and packages a stand-alone executable
go test ./... -v                       # runs tests with verbose output
./shoutrrr/shoutrrr                    # runs the application
```

## Documentation

Shoutrrr's documentation is provided via a GitHub Pages static website.
MkDocs is used to generate the files, via the `./github/workflows/publish-docs.yaml` GitHub workflow, which are deployed to the root directory of the `gh-pages` branch.
GitHub automatically deploys the website upon changes to the `gh-pages` branch.

### Local Development

Ensure that you have first installed the necessary dependencies.
To run the website locally, run the following from the root directory:

```bash
mkdocs serve --config-file build/mkdocs/mkdocs.yaml
```

Note: Disable the markdown extension pymdownx.snippets > check_paths option if not generating the service config docs via the `scripts\generate-service-config-docs.sh` script.

## Semantic Branch Names

Shoutrrr uses semantic branch naming for structured branch names.

| Type       | Description                                                        | Examples                     |
|------------|--------------------------------------------------------------------|------------------------------|
| `chore`    | updating grunt tasks etc; no production code change                | `chore/update-build-script`  |
| `docs`     | changes to the documentation                                       | `docs/update-readme`         |
| `feat`     | new feature for the user, not a new feature for build script       | `feat/user-authentication`   |
| `fix`      | bug fix for the user, not a fix to a build script                  | `fix/login-error`            |
| `refactor` | refactoring production code, eg. renaming a variable               | `refactor/rename-user-class` |
| `style`    | formatting, missing semi colons, etc; no production code change    | `style/format-components`    |
| `test`     | adding missing tests, refactoring tests; no production code change | `test/add-unit-tests`        |

- If the branch is addressing an issue, then include the issue number in the branch name:

  ```text
  feat/#160-add-wechat-support
  ```

## Conventional Commit Messages

Shoutrrr uses the [Conventional Commits specification](https://www.conventionalcommits.org/en/v1.0.0/#summary) for structured commit messages.

Commit Message Structure:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

## Commit Signing

This project requires all commits to be signed in order for pull requests to be accepted.
As Shoutrrr is a notification library that's leveraged by several open-source projects, this is simply a necessity.
Refer to GitHub's documentation on getting it setup: <https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits>
