# Generic

The Generic service can be used for any target that is not explicitly supported by Shoutrrr, as long as it supports receiving the message via a POST request.
Usually, this requires customization on the receiving end to interpret the payload that it receives, and might not be a viable approach.

Common examples for use with service providers can be found under [examples](../../../examples/home-assistant/index.md).

## Custom headers

You can add additional HTTP headers to your request by adding query variables prefixed with `@` (`@key=value`).

For example, the following URL adds the `Accept-Language: tlh-Piqd` header:

```url
generic://example.com?@acceptLanguage=tlh-Piqd
```

The following adds webhook-specific information:

!!! example
    ```url
    generic://api.example.com/webhook?@Authorization=Bearer%20token123&@X-Custom=value
    ```

!!! note
    Header names are normalized to HTTP header format.
    A capital letter after a lowercase letter or a digit starts a new word, so `acceptLanguage` becomes `Accept-Language`, and the first letter of each word is capitalized, so `x-api-key` becomes `X-Api-Key`.
    Acronyms, digits, and other characters are kept as written, so `X-API-Key` stays `X-API-Key`, while an all-lowercase name such as `acceptlanguage` becomes `Acceptlanguage`.

## JSON template

By using the built-in `JSON` template (`template=json`) you can create a generic JSON payload.
The keys used for `title` and `message` can be overridden by supplying the params/query values `titleKey` and `messageKey`.
The other send params are added to the JSON object as they are.
A param named after one of the [Query/Param Props](#queryparam_props) also sets that prop for the send, whatever its letter case.
An invalid value for a prop fails the send.

!!! example
    ```json
    {
        "title": "Oh no!",
        "message": "The thing happened and now there is stuff all over the area!"
    }
    ```

!!! example
    ```url
    generic://api.example.com/webhook?template=json
    ```

### Custom data fields

When using the JSON template, you can add additional key/value pairs to the JSON object by adding query variables prefixed with `$` (`$key=value`).

!!! example
    Using `generic://example.com?$projection=retroazimuthal` would yield:
    ```json
    {
        "title": "Amazing opportunities!",
        "message": "New map book available for purchase.",
        "projection": "retroazimuthal"
    }
    ```

!!! example
    ```url
    generic://webhook.example.com/alert?$source=shoutrrr
    ```

## Shortcut URL

You can just add `generic+` as a prefix to your target URL to use it with the generic service.

For example, the following URL:

```url
https://example.com/api/v1/postStuff
```

Becomes the following `generic+` URL:

```url
generic+https://example.com/api/v1/postStuff
```

!!! Note
    Query variables in a `generic+` URL work as in a `generic://` URL.
    A variable that names a [Query/Param Prop](#queryparam_props) configures the service, such as `template=json`.
    The others are forwarded to the remote server.
    The one exception is `disabletls`, which has no effect in the query of a `generic+` URL, because its scheme decides whether TLS is used.
    A `disabletls` send param still applies, as described under [TLS](#tls).

!!! example
    ```url
    generic+https://example.com/api/v1/postStuff
    ```

## Forwarded query variables

All query variables that are not listed in the [Query/Param Props](#queryparam_props) section will be forwarded to the target endpoint.
If you need to pass a query variable that _is_ reserved, you can prefix it with two underscores (`__`).

!!! Example
    The URL `generic+https://example.com/api/v1/postStuff?contenttype=text/plain` would send a POST message to `https://example.com/api/v1/postStuff` using the `Content-Type: text/plain` header.
    If instead escaped, `generic+https://example.com/api/v1/postStuff?__contenttype=text/plain` would send a POST message to `https://example.com/api/v1/postStuff?contenttype=text/plain` using the `Content-Type: application/json` header (as it's the default).

!!! example
    ```url
    generic://webhook.example.com/alert?template=json&disabletls=yes&method=POST&titlekey=alert_title&messagekey=alert_message
    ```

## TLS

A `generic://` URL sends over HTTPS unless it sets `disabletls=yes`.
A `generic+` URL sends over HTTP when its scheme is `generic+http://` and over HTTPS otherwise, unless a send param overrides it.
A `disabletls=yes` send param sends that request over HTTP even for a `generic+https://` URL, so the scheme alone does not guarantee encryption.
Plain HTTP leaves the connection unencrypted.

Set `skiptlsverify=yes` when the webhook server's certificate cannot be verified, such as a self-signed one.
The connection stays encrypted, but Shoutrrr does not check which server it talks to.
Use `__skiptlsverify` to forward a `skiptlsverify` query variable to the webhook instead.

Both options can also be passed as send params, which apply to that send only.
A custom HTTP client set through `SetHTTPClient` keeps its own TLS settings, so `skiptlsverify` does not change it.
Shoutrrr logs a warning the first time a send goes out without TLS, and the first time a send skips certificate verification through its own client.

## URL Format

--8<-- "docs/services/specialized/generic/config.md"
