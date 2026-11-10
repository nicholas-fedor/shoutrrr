# Services Overview

## Chat Services

Click on the service for a more thorough explanation.

| Service                                   | URL format                                                                                                                                                                                          |
|-------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Discord](./chat/discord/index.md)        | *discord://__`TOKEN`__@__`WEBHOOK_ID`__[?thread_id=__`THREAD_ID`__]*                                                                                                                                |
| [Google Chat](./chat/googlechat/index.md) | *googlechat://chat.googleapis.com/v1/spaces/__`SPACE_ID`__/messages?key=__`KEY`__&token=__`TOKEN`__*                                                                                                |
| [Hangouts](./chat/hangouts/index.md)*     | *hangouts://chat.googleapis.com/v1/spaces/__`SPACE_ID`__/messages?key=__`KEY`__&token=__`TOKEN`__*                                                                                                  |
| [Lark](./chat/lark/index.md)              | *lark://__`HOST`__/__`TOKEN`__[?secret=__`SECRET`__][&title=__`TITLE`__][&link=__`LINK`__]*                                                                                                         |
| [Matrix](./chat/matrix/index.md)          | *matrix://[__`USER`__]:__`PASSWORD`__@__`HOST`__[:__`PORT`__]/[?rooms=__`ROOM_1`__[,__`ROOM_2`__]]*                                                                                                 |
| [Mattermost](./chat/mattermost/index.md)  | *mattermost://[__`USERNAME`__@]__`HOST`__/__`TOKEN`__[/__`CHANNEL`__]*                                                                                                                              |
| [Rocketchat](./chat/rocketchat/index.md)  | *rocketchat://[__`USERNAME`__@]__`HOST`__/__`INTEGRATION_ID`__/__`TOKEN`__[/__`CHANNEL`__&#124;@__`USER`__]*                                                                                        |
| [Signal](./chat/signal/index.md)†         | *signal://[__`USER`__[:__`PASSWORD`__]@]__`HOST`__[:__`PORT`__]/__`SOURCE_PHONE`__/__`RECIPIENT_1`__[/__`RECIPIENT_2`__/...]*                                                                       |
| [Slack](./chat/slack/index.md)            | *slack://{hook&#124;xoxb}:__`TOKEN`__@__`CHANNEL`__[?botname=__`BOT_NAME`__]*                                                                                                                       |
| [Teams](./chat/teams/index.md)            | *teams://?host=__`WORKFLOW_URL`__*                                                                                                                                                                  |
| [Telegram](./chat/telegram/index.md)      | *telegram://__`TOKEN`__@telegram?chats=__`CHAT_1`__[,__`CHAT_2`__,...]*                                                                                                                             |
| [WeCom](./chat/wecom/index.md)            | *wecom://__`KEY`__*                                                                                                                                                                                 |
| [XMPP](./chat/xmpp/index.md)              | *xmpp://__`USER`__:__`PASSWORD`__@__`HOST`__[:__`PORT`__]/?to=__`JID`__[&rooms=__`ROOM_JID`__]* or *xmpps://__`USER`__:__`PASSWORD`__@__`HOST`__[:__`PORT`__]/?to=__`JID`__[&rooms=__`ROOM_JID`__]* |
| [Zulip Chat](./chat/zulip/index.md)       | *zulip://__`BOT_MAIL`__:__`BOT_KEY`__@__`HOST`__/[?stream=__`STREAM`__][&topic=__`TOPIC`__]*                                                                                                        |

\* Deprecated

† A group ID can contain `/` and span several path segments, ending at the next phone number, username, or group ID. For example, `signal://localhost:8080/+18005550100/group.ABCD/EFGH=/+18005550101` sends to the group `group.ABCD/EFGH=` and the phone number `+18005550101`.

## Push Services

| Service                                         | URL format                                                                                                                         |
|-------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| [Bark](./push/bark/index.md)                    | *bark://:__`DEVICE_KEY`__@__`HOST`__[:__`PORT`__][/__`PATH`__]*                                                                    |
| [Gotify](./push/gotify/index.md)                | *gotify://__`HOST`__[:__`PORT`__][/__`PATH`__]/__`TOKEN`__*                                                                        |
| [Home Assistant](./push/homeassistant/index.md) | *homeassistant://__`LONG_LIVED_TOKEN`__@__`HOST`__[:__`PORT`__][/__`BASE_PATH`__]*                                                 |
| [IFTTT](./push/ifttt/index.md)                  | *ifttt://__`KEY`__/?events=__`EVENT_1`__[,__`EVENT_2`__,...][&value1=__`VALUE_1`__][&value2=__`VALUE_2`__][&value3=__`VALUE_3`__]* |
| [Join](./push/join/index.md)                    | *join://shoutrrr:__`API_KEY`__@join/?devices=__`DEVICE_ID_1`__[,__`DEVICE_ID_2`__,...][&icon=__`ICON`__][&title=__`TITLE`__]*      |
| [MQTT](./push/mqtt/index.md)                    | *mqtt://[[__`USERNAME`__][:__`PASSWORD`__]@]__`HOST`__[:__`PORT`__]/__`TOPIC`__*                                                   |
| [Ntfy](./push/ntfy/index.md)                    | *ntfy://[__`USERNAME`__:__`PASSWORD`__@]__`HOST`__/__`TOPIC`__*                                                                    |
| [Pushbullet](./push/pushbullet/index.md)        | *pushbullet://__`API_TOKEN`__[/__`DEVICE`__/#__`CHANNEL`__/__`EMAIL`__]*                                                           |
| [Pushover](./push/pushover/index.md)            | *pushover://shoutrrr:__`API_TOKEN`__@__`USER_KEY`__/[?devices=__`DEVICE_1`__[,__`DEVICE_2`__,...]]*                                |
| [Signalgrid](./push/signalgrid/index.md)        | *signalgrid://__`CLIENT_KEY`__@__`CHANNEL`__*                                                                                      |

## Incident Services

| Service                                    | URL format                                                                                 |
|--------------------------------------------|--------------------------------------------------------------------------------------------|
| [OpsGenie](./incident/opsgenie/index.md)   | *opsgenie://[__`HOST`__]/__`API_KEY`__[?responders=__`RESPONDER_1`__[,__`RESPONDER_2`__]]* |
| [PagerDuty](./incident/pagerduty/index.md) | *pagerduty://[__`HOST`__[:__`PORT`__]]/__`INTEGRATION_KEY`__[?__`QUERY_PARAMETERS`__]*     |

## Email Services

| Service                        | URL format                                                                                                                                                                          |
|--------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Email](./email/smtp/index.md) | *smtp://[__`USERNAME`__[:__`PASSWORD`__]@]__`HOST`__[:__`PORT`__]/?fromaddress=__`FROM_ADDRESS`__&toaddresses=__`TO_ADDRESS_1`__[,__`TO_ADDRESS_2`__,...][&__`QUERY_PARAMETERS`__]* |

## SMS Services

| Service                         | URL format                                                                                                |
|---------------------------------|-----------------------------------------------------------------------------------------------------------|
| [Twilio](./sms/twilio/index.md) | *twilio://__`ACCOUNT_SID`__:__`AUTH_TOKEN`__@__`FROM_NUMBER`__/__`TO_NUMBER_1`__[/__`TO_NUMBER_2`__/...]* |

## Specialized Services

| Service                                           | Description                                                  |
|---------------------------------------------------|--------------------------------------------------------------|
| [Generic Webhook](./specialized/generic/index.md) | Sends notifications directly to a webhook                    |
| [Logger](./specialized/logger/index.md)           | Writes a notification to a configured Go `log.Logger`        |
| [Notifiarr](./specialized/notifiarr/index.md)     | Sends notifications to Notifiarr for media server management |

## Lists

Settings that take a list, such as Telegram's `chats` or the recipients in a Signal or Twilio URL path, leave out empty entries.
A trailing comma, a doubled comma, or an empty path segment adds nothing, so `chats=@alerts,` sends to `@alerts` alone.
A list whose entries are all empty counts as left out.
