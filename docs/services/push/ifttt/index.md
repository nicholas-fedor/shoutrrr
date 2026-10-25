# IFTTT

## URL Format

!!! info ""
    ifttt://__`key`__/?events=__`event1`__[,__`event2`__,...]&value1=__`value1`__&value2=__`value2`__&value3=__`value3`__

--8<-- "docs/services/push/ifttt/config.md"

## Sending to several events

Each event in the `events` param gets its own request, in order.
An event that fails does not stop the events after it, and the error names every event that failed.
