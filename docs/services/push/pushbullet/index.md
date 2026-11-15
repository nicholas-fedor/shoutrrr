# Pushbullet

## URL Format

!!! info ""
    pushbullet://__`API_TOKEN`__[/__`DEVICE`__/#__`CHANNEL`__/__`EMAIL`__]

--8<-- "docs/services/push/pushbullet/config.md"

## Sending to several targets

Each device, channel, and e-mail address in the URL gets its own push, in order.
A target that fails does not stop the targets after it and the error names every target that failed.

## Delivery

A push counts as sent once Pushbullet accepts it, since Pushbullet answers every failure with an error status.
Shoutrrr does not compare Pushbullet's copy of the push with the message.
