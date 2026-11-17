# Twilio

## URL Format

!!! info ""
    twilio://__`ACCOUNT_SID`__:__`AUTH_TOKEN`__@__`FROM_NUMBER`__/__`TO_NUMBER_1`__[/__`TO_NUMBER_2`__/...]

--8<-- "docs/services/sms/twilio/config.md"

## Getting Started

To use the Twilio SMS service, you need a Twilio account. You can sign up at [twilio.com](https://www.twilio.com/).

### Required Credentials

1. __Account SID__, found on your [Twilio Console Dashboard](https://console.twilio.com/)
2. __Auth Token__, found on your [Twilio Console Dashboard](https://console.twilio.com/)
3. __From Number__, a Twilio phone number, alphanumeric sender ID, or Messaging Service SID from your account
4. __To Number(s)__, one or more recipient phone numbers in [E.164 format](https://www.twilio.com/docs/glossary/what-e164)

An Account SID is `AC` followed by 32 hexadecimal characters, and a URL with any other value is rejected.

### API Keys

Twilio recommends [API keys](https://www.twilio.com/docs/iam/api-keys) over the Auth Token for API requests.
To send with an API key, keep the Account SID as the URL user.
Put the API key secret in place of the Auth Token, and set the API key SID with the `apikey` parameter:

```uri
twilio://AC0123456789abcdef0123456789abcdef:API_KEY_SECRET@+18005550100/+18005550101?apikey=SK0123456789abcdef0123456789abcdef
```

Shoutrrr then authenticates with the API key SID and its secret, and the Account SID names the account the message belongs to.
An API key SID is `SK` followed by 32 hexadecimal characters, and a URL with any other value is rejected.
The API key must be a Main or Standard key, or a Restricted key that may create messages.

### Phone Number Format

Phone numbers should be in [E.164 format](https://www.twilio.com/docs/glossary/what-e164) (e.g. `+18005550100`).
Common formatting characters such as spaces, dashes, parentheses, and dots are stripped from phone numbers, so `+1 (800) 555-0100` and `+1.800.555.0100` are also accepted.

### Multiple Recipients

You can send the same message to multiple recipients by adding additional phone numbers to the URL path:

```uri
twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@+18005550100/+18005550101/+18005550102/+18005550103
```

Each recipient receives an independent API call. If delivery fails for one
recipient, the remaining recipients are still attempted and all errors are
returned together.

### Messaging Service SID

Instead of a phone number, you can use a [Twilio Messaging Service SID](https://www.twilio.com/docs/messaging/services) as the sender.
A Messaging Service SID is `MG` followed by 32 hexadecimal characters, and Shoutrrr sends it as `MessagingServiceSid`:

```uri
twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@MG0123456789abcdef0123456789abcdef/+18005550101
```

This is useful when you want Twilio to manage sender selection, opt-out handling,
or when sending from a pool of numbers.

### Alphanumeric Sender ID

Where Twilio supports it, the sender can be an [alphanumeric sender ID](https://www.twilio.com/docs/glossary/what-alphanumeric-sender-id), such as your company name.
Twilio allows up to 11 letters, digits, and spaces, with at least one letter.
A sender that holds a letter is sent as `From` exactly as written, so `MGMHotels` is a sender ID, not a Messaging Service SID.
A URL host cannot hold a space, so a sender ID with a space cannot be used.

### Message Length

Twilio accepts a message body of up to 1600 characters, and the title counts toward it.
A longer body fails before any request.
Characters outside the GSM-7 character set can make Twilio reject a shorter body, which Twilio reports as error 21617.
A send without a title or message also fails before any request, since Twilio needs a body.

## Examples

The examples use a sample Account SID, API key SID, and Messaging Service SID, which have the format the service checks.
Replace them, `AUTH_TOKEN`, and `API_KEY_SECRET` with the values from your Twilio Console.

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

!!! example "Using an API key"
    ```uri
    twilio://AC0123456789abcdef0123456789abcdef:API_KEY_SECRET@+18005550100/+18005550101?apikey=SK0123456789abcdef0123456789abcdef
    ```

!!! example "SMS with title"
    ```uri
    twilio://AC0123456789abcdef0123456789abcdef:AUTH_TOKEN@+18005550100/+18005550101?title=Server+Alert
    ```

## Optional Parameters

You can optionally specify the __`title`__ parameter in the URL.
When set, the title goes on its own line before the message.
The __`apikey`__ parameter sets the API key SID, as [API Keys](#api_keys) describes.

Please refer to the [Twilio SMS API documentation](https://www.twilio.com/docs/messaging/api/message-resource#create-a-message-resource) for more information.
