# Lark

Send notifications to Lark using a custom bot webhook.

## URL Format

!!! info ""
    lark://__`host`__/__`token`__?secret=__`secret`__&title=__`title`__&link=__`url`__

--8<-- "docs/services/chat/lark/config.md"

- `host`: The bot API host (`open.larksuite.com` for Lark, `open.feishu.cn` for Feishu).
- `token`: The bot webhook token (required).
- `secret`: Optional bot secret for signed requests.
- `title`: Optional message title (switches to post format if set).
- `link`: Optional URL to include as a clickable link in the message.

### Example URL

```url
lark://open.larksuite.com/abc123?secret=xyz789&title=Alert&link=https://example.com
```

## Create a Custom Bot in Lark

Official Documentation: [Custom Bot Guide](https://open.larksuite.com/document/client-docs/bot-v3/add-custom-bot)

1. __Invite the Custom Bot to a Group__:
   a. Open the target group, click `More` in the upper-right corner, and then select `Settings`.
   b. In the `Settings` panel, click `Group Bot`.
   c. Click `Add a Bot` under `Group Bot`.
   d. In the `Add Bot` dialog, locate `Custom Bot` and select it.
   e. Set the bot’s name and description, then click `Add`.
   f. Copy the webhook address and click `Finish`.

2. __Get Host and Token__:
   - For __Lark__: Use `host = open.larksuite.com`.
   - For __Feishu__: Use `host = open.feishu.cn`.
   - The `token` is the last segment of the webhook URL.
    For example, in `https://open.larksuite.com/open-apis/bot/v2/hook/xxxxxxxxxxxxxxxxx`, the token is `xxxxxxxxxxxxxxxxx`.

3. __Get Secret (Optional)__:
   a. In group settings, open the bot list, find your custom bot, and select it to access its configuration.
   b. Under `Security Settings`, enable `Signature Verification`.
   c. Click `Copy` to save the secret.
   d. Click `Save` to apply the changes.
