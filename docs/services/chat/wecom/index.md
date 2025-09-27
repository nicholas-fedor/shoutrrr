# WeCom

Send notifications to WeChat Work (Enterprise WeChat) using webhook bots.

## URL Format

!!! info ""
    wecom://__`key`__

--8<-- "docs/services/chat/wecom/config.md"

- `key`: The webhook key from your WeChat Work bot (required).

### Example URL

```url
wecom://693axxx6-7aoc-4bc4-97a0-0ec2sifa5aaa
```

## Create a Webhook Bot in WeChat Work

Official Documentation: [Webhook Bot Guide](https://developer.work.weixin.qq.com/document/path/99110)

1. __Create a Group Bot__:
   a. Open WeChat Work on PC or Web.
   b. Find the target group for receiving notifications.
   c. Right-click the group and select "Add Group Bot".
   d. In the dialog, click "Create a Bot".
   e. Enter a custom bot name and click "Add".
   f. You will receive a webhook URL.

2. __Get the Webhook Key__:
   - The webhook URL will look like: `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=XXXXXXXXXXXXXXXXXX`
   - The `key` is the value after `?key=` in the URL.

3. __Configure Shoutrrr__:
   - Use the key in the Shoutrrr URL: `wecom://YOUR_WEBHOOK_KEY`

### Message Features

- Supports text messages up to 4096 characters
- Can mention users with `mentioned_list` parameter
- Can mention users by mobile number with `mentioned_mobile_list` parameter

### Example with Mentions

```bash
shoutrrr send "wecom://693axxx6-7aoc-4bc4-97a0-0ec2sifa5aaa" "Alert message" --mentioned_list "@all"
