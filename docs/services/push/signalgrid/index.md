# Signalgrid

Upstream docs: <https://docs.signalgrid.co/api/push-api/>

## URL Format

!!! info ""
    signalgrid://__`client-key`__@__`channel`__[?title=__`title`__&type=__`type`__&critical=__`Yes`__]

--8<-- "docs/services/push/signalgrid/config.md"

## Getting Started

1. Create a [Signalgrid account](https://web.signalgrid.co/)
2. Copy your [client key](https://docs.signalgrid.co/get-started/find-client-key/)
3. Copy a [channel token](https://docs.signalgrid.co/get-started/find-channel-token/)

!!! example "Basic notification"
    ```uri
    signalgrid://CLIENT_KEY@CHANNEL
    ```

## Parameters

- __`title`__: Optional notification title. Omitted from the API request when empty.
- __`type`__: Visual severity. One of `CRIT`, `WARN`, `INFO`, `SUCCESS` (default `INFO`). Lowercase values such as `info` are accepted.
- __`critical`__: When `Yes`/`true`, deliver as a [critical alert](https://docs.signalgrid.co/features/critical-notifications/) that can bypass Do Not Disturb on supported devices.

`type=CRIT` does **not** enable critical delivery. Set `critical=Yes` separately.

!!! example "Critical outage alert"
    ```uri
    signalgrid://CLIENT_KEY@CHANNEL?title=Server+Down&type=CRIT&critical=Yes
    ```
