# Using Shoutrrr as a GitHub Action

## Overview

The Shoutrrr GitHub Action (`nicholas-fedor/shoutrrr-action`) integrates Shoutrrr into your workflows to send notifications using service URLs. It supports all Shoutrrr services and allows dynamic messaging with GitHub context variables.

## Usage

Add the action to your `.github/workflows` YAML file.

```yaml title="Workflow Syntax Example"
- name: Shoutrrr
  uses: nicholas-fedor/shoutrrr-action@v1
  with:
    url: <SERVICE_URL>
    title: <NOTIFICATION_TITLE>
    message: <NOTIFICATION_MESSAGE>
```

| Input     | Description                                                                                   | Required |
|-----------|-----------------------------------------------------------------------------------------------|----------|
| `url`     | The Shoutrrr service URL (e.g., `discord://token@webhookid`). Use secrets for sensitive data. | Yes      |
| `title`   | The notification title (optional, for services that support it).                              | No       |
| `message` | The notification message body.                                                                | Yes      |

!!! Note
    Use GitHub secrets for URLs containing tokens. The action uses `shoutrrr send` internally, supporting all services like `discord`, `slack`, `telegram`, etc. Messages can include GitHub variables (e.g., `${{ github.sha }}`).

## Examples

<!-- markdownlint-disable -->
### Send Notification on Push to Main

!!! Example
    ```yaml title="Deploy Workflow with Shoutrrr"
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
            uses: nicholas-fedor/shoutrrr-action@v1
            with:
              url: ${{ secrets.SHOUTRRR_URL }}
              title: Deployed ${{ github.sha }}
              message: See changes at ${{ github.event.compare }}.
    ```

    ```text title="Expected Output (Success)"
    Notification sent
    ```

### Send on Pull Request with Custom Message

!!! Example
    ```yaml title="PR Workflow with Shoutrrr"
    name: PR Notification
    on:
      pull_request:
        types: [opened, synchronize]

    jobs:
      notify:
        runs-on: ubuntu-latest
        steps:
          - name: Shoutrrr
            uses: nicholas-fedor/shoutrrr-action@v1
            with:
              url: ${{ secrets.DISCORD_URL }}
              title: New PR #${{ github.event.number }}
              message: "${{ github.event.pull_request.title }} by ${{ github.actor }}: ${{ github.event.pull_request.html_url }}"
    ```

    ```text title="Expected Output (Success)"
    Notification sent
    ```

### Send on Failure with Verbose Logging

!!! Example
    ```yaml title="Failure Notification with Verbose"
    name: Build
    on: [push]
    jobs:
      build:
        runs-on: ubuntu-latest
        steps:
          - [Build steps]

          - name: Notify on Failure
            if: failure()
            uses: nicholas-fedor/shoutrrr-action@v1
            with:
              url: ${{ secrets.SLACK_URL }}
              title: Build Failed
              message: "Build failed for ${{ github.ref }}. Check logs: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}"
    ```

    ```text title="Expected Output (Success)"
    Notification sent
    ```
<!-- markdownlint-restore -->

## Notes

- **Error Handling**: If sending fails, the action logs errors and may fail the step. Use `continue-on-error: true` if needed.
- **Parameters**: The action passes `title` and `message` to `shoutrrr send`. For service-specific params, embed them in the URL.
- **Timeouts**: Inherits Shoutrrr's 10-second send timeout.
- **Digest Pinning**: Pin to a specific SHA digest (e.g., `@caad2fd0be5099bbc16825bc8f71f9ff8e544ffe`) to maintain security best practices.
