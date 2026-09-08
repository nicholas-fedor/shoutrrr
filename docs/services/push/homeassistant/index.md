# Home Assistant

Upstream docs: <https://developers.home-assistant.io/docs/api/rest/>

The Home Assistant service sends notifications through the REST API using a long-lived access token.
By default it creates a persistent notification in the Home Assistant frontend.
An optional `service` query parameter can target notify actions such as companion-app notifiers.

## URL Format

!!! info ""
    homeassistant://__`token`__@__`host`__[:__`port`__][/__`basepath`__]

--8<-- "docs/services/push/homeassistant/config.md"

## Getting Started

1. Open your Home Assistant profile.
2. Create a __Long-Lived Access Token__.
3. Use the token as the URL username and your instance hostname as the host.

When the port is omitted, HTTPS uses port `443` and HTTP (`disabletls=yes`) uses port `8123`.

!!! example "HTTPS reverse proxy"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@ha.example.com
    ```

!!! example "LAN HTTP"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@homeassistant.local:8123/?disabletls=yes
    ```

    `disabletls=yes` sends the long-lived Bearer token over HTTP. Use it only on a trusted, isolated network.

!!! example "Replace an existing persistent notification"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@homeassistant.local:8123/?disabletls=yes&nid=watchtower&title=Update
    ```

    `disabletls=yes` sends the long-lived Bearer token over HTTP. Use it only on a trusted, isolated network.

!!! example "Companion app notifier"
    ```uri
    homeassistant://LONG_LIVED_TOKEN@ha.example.com?service=notify.mobile_app_phone
    ```

## Parameters

- __`title`__: Optional notification title. Omitted from the request when empty.
- __`service`__: Home Assistant action. Empty defaults to `persistent_notification.create`. A value without a dot is sent as `notify/<value>`. A value with a dot is sent as `<domain>/<action>`.
- __`targets`__: Comma-separated notify destinations. Sent as the Home Assistant `target` field and omitted for persistent notifications.
- __`nid`__: Persistent notification ID. When set, Home Assistant overwrites the notification with that ID. Omitted when empty.
- __`disabletls`__: Use HTTP instead of HTTPS.
- __`skiptlsverify`__: Skip TLS certificate verification. Use only with self-signed certificates.

The request is `POST /api/services/{domain}/{service}` with `Authorization: Bearer <token>` and a JSON body containing `message` plus any optional fields above.

Webhook triggers that fire automations are not part of this service. Those endpoints do not use a long-lived access token.
