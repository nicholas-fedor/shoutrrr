# IFTTT

## URL Format

!!! info ""
    ifttt://__`KEY`__/?events=__`EVENT_1`__[,__`EVENT_2`__,...][&value1=__`VALUE_1`__][&value2=__`VALUE_2`__][&value3=__`VALUE_3`__]

--8<-- "docs/services/push/ifttt/config.md"

## Sending to several events

Each event in the `events` param gets its own request, in order.
An event that fails does not stop the events after it, and the error names every event that failed.
An empty event name, as in `events=a,`, is rejected before any request.
