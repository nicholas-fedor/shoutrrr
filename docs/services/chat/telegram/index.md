# Telegram

## URL Format

!!! info ""
    telegram://__`token`__@telegram?chats=__`channel-1`__[,__`chat-id-1`__,...]

--8<-- "docs/services/chat/telegram/config.md"

## Getting a token for Telegram

Talk to [the botfather](https://core.telegram.org/bots#6-botfather).

## Identifying the target chats/channels

The `chats` param consists of one or more `Chat ID`s or `channel name`s.

### Public Channels

The channel names can be retrieved in the telegram client in the `Channel info` section for public channels.
Replace the `t.me/` prefix from the link with a `@`.

!!! note
    Channels names need to be prefixed by `@` to identify them as such.

!!! note
    If your channel only has an invite link (starting with `t.me/+`), you have to use it's Chat ID (see below)

!!! note
    A `message_thread_id` param ([reference](https://core.telegram.org/bots/api#sendmessage)) can be added, with the format of `$chat_id:$message_thread_id`. [More info](https://stackoverflow.com/questions/74773675/how-to-get-topic-id-for-telegram-group-chat/75178418#75178418) on how to obtain the `message_thread_id`.

### Chats

Private channels, Group chats and private chats are identified by `Chat ID`s. Unfortunatly, they are generally not visible in the
telegram clients.
The easiest way to retrieve them is by using the `shoutrrr generate telegram` command which will guide you through
creating a URL with your target chats.

!!! tip
    You can use the `nickfedor/shoutrrr` image in docker to run it without download/installing the `shoutrrr` CLI using:

    ```
    docker run --rm -it nickfedor/shoutrrr generate telegram
    ```

### Asking @shoutrrrbot

Another way of retrieving the Chat IDs, is by forwarding a message from the target chat to the [@shoutrrrbot](https://t.me/shoutrrrbot).
It will reply with the Chat ID for the chat where the forwarded message was originally posted.
Note that it will not work correctly for Group chats, as those messages are just seen as being posted by a user, not in a specific chat.
Instead you can use the second method, which is to invite the @shoutrrrbot into your group chat and address a message to it (start the message with @shoutrrrbot). You can then safely kick the bot from the group.

The bot should be constantly online, unless it's usage exceeds the free tier on GCP. It's source is available at [github.com/nicholas-fedor/shoutrrrbot](https://github.com/nicholas-fedor/shoutrrrbot).

## Sending to several chats

Each chat in the `chats` param gets its own request, in order.
A chat that Telegram rejects does not stop the chats after it, and the error lists every chat that failed.

A message can hold up to 4096 characters, counted as UTF-16 code units the way Telegram measures text.
Most characters count as one unit, and an emoji or another character outside the Basic Multilingual Plane counts as two.
A title sent on its own line, as the HTML parse mode or no parse mode sends it, counts toward the limit too.
A longer message fails before any request is sent.

## Optional parameters

You can optionally specify the __`notification`__, __`parseMode`__ and __`preview`__ parameters in the URL:

!!! info ""
    <pre>telegram://__`token`__@__`telegram`__/?channels=__`channel`__&notification=no&preview=false&parseMode=html</pre>

See [the telegram documentation](https://core.telegram.org/bots/api#sendmessage) for more information.

!!! note
    `preview` and `notification` are inverted in regards to their API counterparts (`disable_web_page_preview` and `disable_notification`)

### Parse Mode and Title

If a parse mode is specified, the message needs to be escaped as per the corresponding sections in
[Formatting options](https://core.telegram.org/bots/api#formatting-options).

When a title has been specified, it will be prepended to the message, but this is only supported for
the `HTML` parse mode. Note that, if no parse mode is specified, the message will be escaped and sent using `HTML`.

Since the markdown modes are really hard to escape correctly, it's recommended to stick to `HTML` parse mode.

## Bot API server

By default, Shoutrrr sends requests to Telegram's own Bot API server at `https://api.telegram.org`.
The `apiurl` param points it at another server, such as a [self-hosted Bot API server](https://github.com/tdlib/telegram-bot-api) or a proxy where Telegram's domains are blocked.
Requests go to the given base URL followed by `/bot<token>/<method>`, so a proxy can serve the API under a path.

!!! example "Self-hosted Bot API server"
    ```uri
    telegram://token@telegram?chats=@channel&apiurl=http://localhost:8081
    ```

The URL must use `http` or `https` and have a host.
It cannot hold userinfo, a query, or a fragment.
The self-hosted server accepts only plain HTTP, so use `https` only behind a TLS proxy.

Every request carries the bot token in its path, so an `http` API URL sends the token unencrypted.
Shoutrrr logs a warning the first time a send goes out that way.

Set `skiptlsverify=yes` when the TLS proxy's certificate cannot be verified, such as a self-signed one.
The connection stays encrypted, but Shoutrrr does not check which server it talks to.
The option can also be passed as a send param, which applies to that send only.
A custom HTTP client set through `SetHTTPClient` keeps its own TLS settings, so `skiptlsverify` does not change it.
Shoutrrr logs a warning the first time a send skips certificate verification through its own client.

To generate a URL for another server, pass the params to the generator:

```bash
shoutrrr generate telegram -g telegram -p apiurl=http://localhost:8081
shoutrrr generate telegram -g telegram -p apiurl=https://tg.example.com -p skiptlsverify=yes
```
