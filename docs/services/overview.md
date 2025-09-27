# Services Overview

## Chat Services

Click on the service for a more thorough explanation.

| Service                                   | URL format                                                                                                                  |
|-------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|
| [Discord](./chat/discord/index.md)        | *discord://__`token`__@__`id`__[?thread_id=__`threadid`__]*                                                                 |
| [Google Chat](./chat/googlechat/index.md) | *googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz*                                                 |
| [Hangouts](./chat/hangouts/index.md)*     | *hangouts://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz*                                                   |
| [Lark](./chat/lark/index.md)              | *lark://__`host`__/__`token`__?secret=__`secret`__&title=__`title`__&link=__`url`__*                                        |
| [Matrix](./chat/matrix/index.md)          | *matrix://__`username`__:__`password`__@__`host`__:__`port`__/[?rooms=__`!roomID1`__[,__`roomAlias2`__]]*                   |
| [Mattermost](./chat/mattermost/index.md)  | *mattermost://[__`username`__@]__`mattermost-host`__/__`token`__[/__`channel`__]*                                           |
| [Rocketchat](./chat/rocketchat/index.md)  | *rocketchat://[__`username`__@]__`rocketchat-host`__/__`token`__[/__`channel`&#124;`@recipient`__]*                         |
| [Signal](./chat/signal/index.md)          | *signal://[__`user`__[:__`password`__]@]__`host`__[:__`port`__]/__`source_phone`__/__`recipient1`__[,__`recipient2`__,...]* |
| [Slack](./chat/slack/index.md)            | *slack://[__`botname`__@]__`token-a`__/__`token-b`__/__`token-c`__*                                                         |
| [Teams](./chat/teams/index.md)            | *teams://__`group`__@__`tenant`__/__`altId`__/__`groupOwner`__?host=__`organization`__.webhook.office.com*                  |
| [Telegram](./chat/telegram/index.md)      | *telegram://__`token`__@telegram?chats=__`@channel-1`__[,__`chat-id-1`__,...]*                                              |
| [WeCom](./chat/wecom/index.md)            | *wecom://__`key`__*                                                                                                         |
| [Zulip Chat](./chat/zulip/index.md)       | *zulip://__`bot-mail`__:__`bot-key`__@__`zulip-domain`__/?stream=__`name-or-id`__&topic=__`name`__*                         |

\* Deprecated

## Push Services

| Service                                  | URL format                                                                                                              |
|------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|
| [Bark](./push/bark/index.md)             | *bark://__`devicekey`__@__`host`__*                                                                                     |
| [Gotify](./push/gotify/index.md)         | *gotify://__`gotify-host`__/__`token`__*                                                                                |
| [IFTTT](./push/ifttt/index.md)           | *ifttt://__`key`__/?events=__`event1`__[,__`event2`__,...]&value1=__`value1`__&value2=__`value2`__&value3=__`value3`__* |
| [Join](./push/join/index.md)             | *join://shoutrrr:__`api-key`__@join/?devices=__`device1`__[,__`device2`__, ...][&icon=__`icon`__][&title=__`title`__]*  |
| [Ntfy](./push/ntfy/index.md)             | *ntfy://__`username`__:__`password`<__@ntfy.sh>/__`topic`__*                                                            |
| [Pushbullet](./push/pushbullet/index.md) | *pushbullet://__`api-token`__[/__`device`__/#__`channel`__/__`email`__]*                                                |
| [Pushover](./push/pushover/index.md)     | *pushover://shoutrrr:__`apiToken`__@__`userKey`__/?devices=__`device1`__[,__`device2`__, ...]*                          |

## Incident Services

| Service                              | URL format                                                                                                                                                          |
|--------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [OpsGenie](./incident/opsgenie/index.md)      | *opsgenie://__`host`__/token?responders=__`responder1`__[,__`responder2`__]*                                                                                        |

## Email Services

| Service                              | URL format                                                                                                                                                          |
|--------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Email](./email/smtp/index.md)            | *smtp://__`username`__:__`password`__@__`host`__:__`port`__/?fromaddress=__`fromAddress`__&toaddresses=__`recipient1`__[,__`recipient2`__,...][&additional_params]* |

## Specialized Services

| Service                                           | Description                                           |
|---------------------------------------------------|-------------------------------------------------------|
| [Logger](./specialized/logger/index.md)           | Writes a notification to a configured Go `log.Logger` |
| [Generic Webhook](./specialized/generic/index.md) | Sends notifications directly to a webhook             |
