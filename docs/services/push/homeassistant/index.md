# Home Assistant

Upstream docs: <https://developers.home-assistant.io/docs/api/rest/>

The Home Assistant service sends notifications through the REST API using a long-lived access token.
By default it creates a persistent notification in the Home Assistant frontend.
An optional `service` query parameter can target notify actions such as companion-app notifiers.

Requests always use HTTPS with server certificate verification.

## URL Format

!!! info ""
    homeassistant://__`token`__@__`host`__[:__`port`__][/__`basepath`__]

--8<-- "docs/services/push/homeassistant/config.md"

## Getting Started

1. Open your Home Assistant profile.
2. Create a __Long-Lived Access Token__.
3. Use the token as the URL username and your instance hostname as the host.

When the port is omitted, HTTPS uses port `443`. Home Assistant listening on 8123 with TLS uses an explicit port:

!!! example "HTTPS reverse proxy"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@ha.example.com
    ```

!!! example "HTTPS on port 8123"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@homeassistant.local:8123
    ```

!!! example "Replace an existing persistent notification"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@ha.example.com?nid=watchtower&title=Update
    ```

!!! example "Companion app notifier"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@ha.example.com?service=notify.mobile_app_phone
    ```

## Parameters

Endpoint and transport:

- __`service`__: Home Assistant action. Empty defaults to `persistent_notification.create`. A value without a dot is sent as `notify/<value>`. A value with a dot is sent as `<domain>/<action>`.

JSON body:

- __`title`__: Optional notification title. Omitted from the request when empty.
- __`targets`__: Comma-separated notify destinations. Sent as the Home Assistant `target` field and omitted for persistent notifications.
- __`nid`__: Persistent notification ID. Sent as `notification_id`. When set, Home Assistant overwrites the notification with that ID. Omitted when empty.

The request is `POST /api/services/{domain}/{service}` with `Authorization: Bearer <token>`.

The JSON body contains `message` and the optional fields `title`, `notification_id`, and `target`. It does not include `service`.

Webhook triggers that fire automations are not part of this service. Those endpoints do not use a long-lived access token.
