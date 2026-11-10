# Pushover

## URL Format

!!! info ""
    pushover://shoutrrr:__`API_TOKEN`__@__`USER_KEY`__/[?devices=__`DEVICE_1`__[,__`DEVICE_2`__,...]]

--8<-- "docs/services/push/pushover/config.md"

## Getting the keys from Pushover

At your [Pushover dashboard](https://pushover.net/) you can view your __`USER_KEY`__ in the top right.
![Screenshot 1](po-1.png)

The `Name` column of the device list is what is used to refer to your devices (__`DEVICE_1`__ etc.)
![Screenshot 4](po-4.png)

At the bottom of the same page there are links your _applications_, where you can find your __`API_TOKEN`__
![Screenshot 2](po-2.png)

The __`API_TOKEN`__ is displayed at the top of the application page.
![Screenshot 3](po-3.png)

## Optional parameters

You can optionally specify the __`title`__ and __`priority`__ parameters in the URL:

!!! info ""
    pushover://shoutrrr:__`API_TOKEN`__@__`USER_KEY`__/?devices=__`DEVICE`__&title=Custom+Title&priority=1

The priority is a number from -2 (lowest) to 2 (emergency), and other values are rejected.

### Emergency priority

Priority 2 sends an emergency notification, which Pushover repeats until the user acknowledges it.
The `retry` param sets the time between repeats and the `expire` param sets how long they continue.
Both are durations in whole seconds, such as `30s`, `15m`, or `1h`.
`retry` must be at least `30s` and defaults to `15m`.
`expire` must be at most `3h` and defaults to `1h`.
Pushover stops after 50 repeats, whatever `expire` says.
They are sent only with emergency priority.

!!! Example "Emergency notification that repeats every minute for half an hour"

    ```uri
    pushover://shoutrrr:API_TOKEN@USER_KEY/?priority=2&retry=1m&expire=30m
    ```

### Message time

Pushover shows and orders messages by the time its servers received them, so messages sent close together can appear out of order.
The `timestamp` param sets the time to show instead, as a Unix time in seconds, such as the time the event happened.
It is usually set for each send, as in `params["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)`, but the URL can also set it.

The timestamp must not be negative, and `0` leaves the time to Pushover.
End-to-end encryption does not cover it, so it is always sent as is.

### Message level

A sender can set a message level with the `level` param, as `Params.SetLevel` does.
When neither the URL nor the send params set `priority`, the level picks the priority:

| Level   | Priority   |
|---------|------------|
| Debug   | -1 (low)   |
| Info    | 0 (normal) |
| Warning | 0 (normal) |
| Error   | 1 (high)   |

High priority bypasses quiet hours, so only errors use it.
A `priority` in the URL or the send params takes precedence over the level, even when it is set to the default.

## End-to-end encryption

Pushover [end-to-end encryption](https://pushover.net/api#e2ee) encrypts `message` and `title` on the client before they are sent.
Generate a 256-bit key and configure the same value in the Pushover iOS or Android app:

```bash
openssl rand -hex 32
```

Then include it in the service URL:

!!! info ""
    pushover://shoutrrr:__`API_TOKEN`__@__`USER_KEY`__/?encryptionkey=__`ENCRYPTION_KEY`__

Replace `ENCRYPTION_KEY` with the 64 hexadecimal characters that the command prints.

`key` is accepted as an alias for `encryptionkey`.
A valid key enables encryption automatically.
An invalid key fails during URL verification, and encryption errors refuse to send rather than falling back to plaintext.

Encrypted fields are larger than the original text and still count toward Pushover's 1024-character field limit.
Desktop clients cannot decrypt end-to-end encrypted messages.

Please refer to the [Pushover API documentation](https://pushover.net/api#messages) for more information.
