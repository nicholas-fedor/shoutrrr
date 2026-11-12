# Join

## URL Format

!!! info ""
    join://shoutrrr:__`API_KEY`__@join/?devices=__`DEVICE_ID_1`__[,__`DEVICE_ID_2`__,...][&icon=__`ICON`__][&title=__`TITLE`__]

--8<-- "docs/services/push/join/config.md"

## Guide

1. Go to the [Join Webapp](https://joinjoaomgcd.appspot.com/)
2. Select your device
3. Click **Join API**
4. Your `deviceId` is shown in the top
5. Click **Show** next to `API Key` to see your key
6. Your Shoutrrr URL will then be:
    `join://shoutrrr:`__`API_KEY`__`@join/?devices=`__`DEVICE_ID`__

!!! note ""
    Multiple `deviceId`s can be combined with a `,` (repeat steps 2-4).

## Errors

Join answers a push with HTTP 200 even when it refuses it.
Its reply says whether it accepted the push, so Shoutrrr reads the reply to decide whether the send failed.

| Join's reply | Result |
| --- | --- |
| HTTP 200 that reports success | The send succeeds. |
| HTTP 200 whose body is not JSON | The send succeeds. |
| HTTP 200 that reports a refusal | The send fails with Join's reason. |
| HTTP 200 that ends before its whole body is read | The send fails, since the rest could hold a refusal. |
| Any other status | The send fails with the status, and with Join's reason when the reply gives one. |

These are the reasons Join gives most often.

| Reason | Cause |
| --- | --- |
| `No device to send message to` | The API key is wrong. |
| `No devices to send to` | The device IDs are unknown. |
