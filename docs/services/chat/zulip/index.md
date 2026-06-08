# Zulip Chat

[Zulip](https://zulip.com/) is an open-source group chat application that supports both live and asynchronous conversations.
Shoutrrr's Zulip service sends notifications to Zulip streams using bot credentials.

## URL Format

!!! info ""
    zulip://__`botmail`__:__`botkey`__@__`host`__/?stream=__`stream`__&topic=__`topic`__

--8<-- "docs/services/chat/zulip/config.md"

!!! note
    When constructing the service URL manually, the `@` in the bot e-mail address must be
    URL-encoded as `%40`. When using the Go API or the playground, this is handled automatically.

!!! note
    The __`host`__ field may include a non-standard port (e.g., `zulip.example.com:8443`).
    When using the default HTTPS port (443), the port can be omitted.

## Bot Setup

To use Shoutrrr with Zulip, you need to create a bot in your Zulip organization:

1. Go to __Settings__ → __Personal settings__ → __Bots__
2. Click __Add a new bot__
3. Choose __Generic bot__ as the bot type
4. Fill in the bot name and optionally an avatar
5. Click __Create bot__
6. Copy the bot's __email__ and __API key__

Use the bot email as the `botmail` and the API key as the `botkey` in your service URL.

## Stream and Topic

Both `stream` and `topic` are optional. They can be provided in the service URL or overridden
at send time using `types.Params`:

- __Stream__: The name of the Zulip stream to send messages to (e.g., `general`, `alerts`)
- __Topic__: The message topic within the stream (e.g., `server-monitoring`). Zulip topics keep
  conversations organized within a stream.
- __Title__: A notification title that is prepended to the message content. If no `topic` is
  set, the `title` is used as the topic instead.

If neither is specified, the message is sent to the default stream configured for the bot.

## Examples

!!! example "Basic notification (default stream)"
    ```uri
    zulip://my-bot%40zulipchat.com:correcthorsebatterystable@example.zulipchat.com
    ```

!!! example "With stream and topic in URL"
    ```uri
    zulip://my-bot%40zulipchat.com:correcthorsebatterystable@example.zulipchat.com?stream=alerts&topic=monitoring
    ```

!!! example "With custom port"
    ```uri
    zulip://my-bot%40zulipchat.com:correcthorsebatterystable@example.zulipchat.com:8443?stream=general
    ```

!!! example "Override stream and topic at send time"
    ```go
    sender, _ := shoutrrr.CreateSender(url)

    params := make(types.Params)
    params["stream"] = "alerts"
    params["topic"] = "disk-space-warning"

    sender.Send("Disk usage exceeded 90% on server-01", &params)
    ```

!!! example "Add a notification title to the message"
    ```go
    params := make(types.Params)
    params["title"] = "Deployment Notification"

    sender.Send("Deployed v2.3.1 to production", &params)
    ```

!!! example "Override topic at send time"
    ```go
    params := make(types.Params)
    params["topic"] = "deployment-notification"

    sender.Send("Deployed v2.3.1 to production", &params)
    ```

!!! example "Using the CLI"
    ```bash
    shoutrrr send --url "zulip://bot%40example.com:api-key@zulip.example.com?stream=alerts" --message "Server restarted"
    ```

## Direct Messages

To send a direct message, set the `type` parameter to `direct` and provide recipients via the `to` parameter (comma-separated emails or user IDs). The `stream` value can serve as a fallback recipient list for direct messages. The `topic` parameter is ignored for direct messages (only channels use topics).

!!! example "Direct message via params"
    ```go
    params := make(types.Params)
    params["type"] = "direct"
    params["to"] = "user1@example.com,user2@example.com"

    sender.Send("Hello via DM", &params)
    ```

!!! example "Direct message service URL"
    ```
    zulip://my-bot%40zulipchat.com:correcthorsebatterystable@example.zulipchat.com?type=direct&to=user1@example.com,user2@example.com
    ```

## Server-Side Limits

The service fetches `max_message_length` and `max_topic_length` from Zulip's register endpoint on the first send (with 10s timeout). If the request fails or returns zero values, it falls back to 10,000 bytes for content and 60 characters for topics. Length checks use rune count for topics (Unicode aware) and byte count for message content.
