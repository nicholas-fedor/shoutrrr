# Home Assistant

## Overview

Home Assistant can receive Shoutrrr notifications in two ways.

Use the dedicated `homeassistant` service to create persistent notifications or call notify actions through the REST API with a long-lived access token.

Use a webhook URL only when an automation should run from an unauthenticated `POST` to `/api/webhook/<webhook_id>`.

## REST notifications

Create a long-lived access token in your Home Assistant profile, then send to the REST API over HTTPS.

```url title="Home Assistant REST URL"
homeassistant://<LONG_LIVED_TOKEN>@<HA_HOST>
```

!!! Example
    ```bash title="Send a persistent notification"
    shoutrrr send --url "homeassistant://LONG_LIVED_TOKEN@ha.example.com?title=Update" --message "Hello, Home Assistant!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

Reuse `nid` when later messages should replace the same persistent notification instead of stacking.

```url
homeassistant://LONG_LIVED_TOKEN@ha.example.com?nid=watchtower
```

Call a notify action such as a companion-app notifier with `service`:

```url
homeassistant://LONG_LIVED_TOKEN@ha.example.com?service=notify.mobile_app_phone
```

## Webhook trigger

Configure a webhook trigger in Home Assistant, then POST JSON to `/api/webhook/<WEBHOOK_ID>`.
In the automation, read the message from `{{ trigger.json.message }}`.

```url title="Generic Service URL for HTTPS"
generic://<HA_IP_ADDRESS>:<HA_PORT>/api/webhook/<WEBHOOK_ID>?template=json
```

!!! Example
    ```bash title="Send Command to a webhook"
    shoutrrr send --url "generic://192.168.1.100:8123/api/webhook/abc123?template=json" --message "Hello, Home Assistant!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

!!! Note
    Webhook IDs are unauthenticated. Treat them like passwords and keep `local_only` enabled unless internet access is required.
