# Home Assistant

## Overview

This example demonstrates how to configure the Shoutrrr `generic` service to send notifications to Home Assistant via its webhook API.

## Usage

Configure the `generic` service URL to target Home Assistant's webhook endpoint. The URL requires the Home Assistant IP address, port, and webhook ID.

=== "HTTPS (Default)"

    ```url title="Generic Service URL for HTTPS"
    generic://<HA_IP_ADDRESS>:<HA_PORT>/api/webhook/<WEBHOOK_ID>?template=json
    ```

=== "HTTP"

    ```url title="Generic Service URL for HTTP"
    generic://<HA_IP_ADDRESS>:<HA_PORT>/api/webhook/<WEBHOOK_ID>?template=json&disabletls=yes
    ```

!!! Note
    Replace `<HA_IP_ADDRESS>`, `<HA_PORT>`, and `<WEBHOOK_ID>` with your Home Assistant instance details. In Home Assistant, use `{{ trigger.json.message }}` to extract the message from the JSON payload sent by Shoutrrr.

## Example

<!-- markdownlint-disable -->
### Send Notification to Home Assistant

!!! Example
    ```bash title="Send Command to Home Assistant"
    shoutrrr send --url "generic://192.168.1.100:8123/api/webhook/abc123?template=json" --message "Hello, Home Assistant!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Send Notification with HTTP and Verbose Output

!!! Example
    ```bash title="Send Command with HTTP and Verbose"
    shoutrrr send --url "generic://192.168.1.100:8123/api/webhook/abc123?template=json&disabletls=yes" --message "Hello, Home Assistant!" --verbose
    ```

    ```text title="Expected Output"
    URLs: generic://192.168.1.100:8123/api/webhook/abc123?template=json&disabletls=yes
    Message: Hello, Home Assistant!
    Notification sent
    ```
<!-- markdownlint-restore -->

## Notes

- **Webhook Setup**: Create a webhook in Home Assistant to obtain the `WEBHOOK_ID`.
- **Template**: The `template=json` query parameter ensures the message is sent as a JSON payload.
- **Accessing Message**: Use `{{ trigger.json.message }}` in Home Assistant automations to retrieve the message.
- **Credit**: Example inspired by [@JeffCrum1](https://github.com/JeffCrum1), [Issue #325](https://github.com/containrrr/shoutrrr/issues/325#issuecomment-1460105065).
