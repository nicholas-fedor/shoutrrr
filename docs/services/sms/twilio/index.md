# Twilio

## URL Format

!!! info ""
    twilio://__`ACCOUNT_SID`__:__`AUTH_TOKEN`__@__`FROM_NUMBER`__/__`TO_NUMBER_1`__[/__`TO_NUMBER_2`__/...]

--8<-- "docs/services/sms/twilio/config.md"

## Getting Started

To use the Twilio SMS service, you need a Twilio account. You can sign up at [twilio.com](https://www.twilio.com/).

### Required Credentials

1. **Account SID** — Found on your [Twilio Console Dashboard](https://console.twilio.com/)
2. **Auth Token** — Found on your [Twilio Console Dashboard](https://console.twilio.com/)
3. **From Number** — A Twilio phone number **or** Messaging Service SID from your account
4. **To Number(s)** — One or more recipient phone numbers in [E.164 format](https://www.twilio.com/docs/glossary/what-e164)

An Account SID is `AC` followed by 32 hexadecimal characters, and a URL with any other value is rejected.

### Phone Number Format

Phone numbers should be in [E.164 format](https://www.twilio.com/docs/glossary/what-e164) (e.g. `+18005550100`).
Common formatting characters such as spaces, dashes, parentheses, and dots are
automatically stripped, so `+1 (800) 555-0100` and `+1.800.555.0100` are also accepted.

### Multiple Recipients

You can send the same message to multiple recipients by adding additional phone numbers to the URL path:

```uri
twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@+18005550100/+18005550101/+18005550102/+18005550103
```

Each recipient receives an independent API call. If delivery fails for one
recipient, the remaining recipients are still attempted and all errors are
returned together.

### Messaging Service SID

Instead of a phone number, you can use a [Twilio Messaging Service SID](https://www.twilio.com/docs/messaging/services) as the sender. Messaging Service SIDs start with `MG`:

```uri
twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@MG0123456789abcdef0123456789abcdef/+18005550101
```

This is useful when you want Twilio to manage sender selection, opt-out handling,
or when sending from a pool of numbers.

## Examples

The examples use a sample Account SID and Messaging Service SID, which have the format the service checks.
Replace them and `AUTH_TOKEN` with the values from your Twilio Console.

!!! example "Basic SMS"
    ```uri
    twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@+18005550100/+18005550101
    ```

!!! example "Multiple recipients"
    ```uri
    twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@+18005550100/+18005550101/+18005550102
    ```

!!! example "Using a Messaging Service SID"
    ```uri
    twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@MG0123456789abcdef0123456789abcdef/+18005550101
    ```

!!! example "SMS with title"
    ```uri
    twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@+18005550100/+18005550101?title=Server+Alert
    ```

## Optional Parameters

You can optionally specify the __`title`__ parameter in the URL. When set, the title is prepended to the message body.

Please refer to the [Twilio SMS API documentation](https://www.twilio.com/docs/sms/api/message-resource#create-a-message-resource) for more information.
