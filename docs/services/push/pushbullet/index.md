# Pushbullet

## URL Format

!!! info ""
    pushbullet://__`api-token`__[/__`device`__/#__`channel`__/__`email`__]

--8<-- "docs/services/push/pushbullet/config.md"

## Sending to several targets

Each device, channel, and e-mail address in the URL gets its own push, in order.
A target that fails does not stop the targets after it and the error names every target that failed.
