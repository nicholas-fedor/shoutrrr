# Completion

## Overview

The `completion` command generates a completion script for the specified shell.

## Usage

```bash title="Completion Command Syntax"
shoutrrr completion [SHELL]
```

## Available Options

| Shell        | Description                                       |
|--------------|---------------------------------------------------|
| `bash`       | Generate the completion script for the Bash shell |
| `fish`       | Generate the completion script for the Fish shell |
| `powershell` | Generate the completion script for PowerShell     |
| `zsh`        | Generate the completion script for the ZSH shell  |

## Completion Script Installation

### Bash

1. Save the completion script depending on your operating system and Bash configuration:

    ```bash
    shoutrrr completions bash | sudo tee /usr/share/bash-completion/completions/shoutrrr >/dev/null
    ```

2. Reload the Bash configuration to make it available to the current shell session:

    ```bash
    source ~/.bashrc
    ```

### Windows PowerShell

1. Save the completion script to a location of your preference:

    ```powershell
    Invoke-Expression "shoutrrr.exe completions powershell | Out-File -FilePath $HOME\Documents\PowerShell\Scripts\shoutrrr_completion.ps1"
    ```

2. Invoke the completion script within your PowerShell profile:

    ```powershell
    Add-Content -Path $PROFILE -Value '. $HOME\Documents\PowerShell\Scripts\shoutrrr.ps1'
    ```

3. Reload your PowerShell profile to invoke the change:

    ```powershell
    . $PROFILE
    ```
