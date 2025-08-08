# Services Overview

## Available Services

Click on the service for a more thorough explanation.

| Service                              | URL format                                                                                                                                                          |
|--------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Bark](./bark/index.md)              | *bark://__`devicekey`__@__`host`__*                                                                                                                                 |
| [Discord](./discord/index.md)        | *discord://__`token`__@__`id`__[?thread_id=__`threadid`__]*                                                                                                         |
| [Email](./email/index.md)            | *smtp://__`username`__:__`password`__@__`host`__:__`port`__/?fromaddress=__`fromAddress`__&toaddresses=__`recipient1`__[,__`recipient2`__,...][&additional_params]* |
| [Google Chat](./googlechat/index.md) | *googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz*                                                                                         |
| [Gotify](./gotify/index.md)          | *gotify://__`gotify-host`__/__`token`__*                                                                                                                            |
| [Hangouts](./hangouts/index.md)*     | *hangouts://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz*                                                                                           |
| [IFTTT](./ifttt/index.md)            | *ifttt://__`key`__/?events=__`event1`__[,__`event2`__,...]&value1=__`value1`__&value2=__`value2`__&value3=__`value3`__*                                             |
| [Join](./join/index.md)              | *join://shoutrrr:__`api-key`__@join/?devices=__`device1`__[,__`device2`__, ...][&icon=__`icon`__][&title=__`title`__]*                                              |
| [Lark](./lark/index.md)              | *lark://__`host`__/__`token`__?secret=__`secret`__&title=__`title`__&link=__`url`__*                                                                                |
| [Matrix](./matrix/index.md)          | *matrix://__`username`__:__`password`__@__`host`__:__`port`__/[?rooms=__`!roomID1`__[,__`roomAlias2`__]]*                                                           |
| [Mattermost](./mattermost/index.md)  | *mattermost://[__`username`__@]__`mattermost-host`__/__`token`__[/__`channel`__]*                                                                                   |
| [Ntfy](./ntfy/index.md)              | *ntfy://__`username`__:__`password`<__@ntfy.sh>/__`topic`__*                                                                                                        |
| [OpsGenie](./opsgenie/index.md)      | *opsgenie://__`host`__/token?responders=__`responder1`__[,__`responder2`__]*                                                                                        |
| [Pushbullet](./pushbullet/index.md)  | *pushbullet://__`api-token`__[/__`device`__/#__`channel`__/__`email`__]*                                                                                            |
| [Pushover](./pushover/index.md)      | *pushover://shoutrrr:__`apiToken`__@__`userKey`__/?devices=__`device1`__[,__`device2`__, ...]*                                                                      |
| [Rocketchat](./rocketchat/index.md)  | *rocketchat://[__`username`__@]__`rocketchat-host`__/__`token`__[/__`channel`&#124;`@recipient`__]*                                                                 |
| [Slack](./slack/index.md)            | *slack://[__`botname`__@]__`token-a`__/__`token-b`__/__`token-c`__*                                                                                                 |
| [Teams](./teams/index.md)            | *teams://__`group`__@__`tenant`__/__`altId`__/__`groupOwner`__?host=__`organization`__.webhook.office.com*                                                          |
| [Telegram](./telegram/index.md)      | *telegram://__`token`__@telegram?chats=__`@channel-1`__[,__`chat-id-1`__,...]*                                                                                      |
| [Zulip Chat](./zulip/index.md)       | *zulip://__`bot-mail`__:__`bot-key`__@__`zulip-domain`__/?stream=__`name-or-id`__&topic=__`name`__*                                                                 |
| \* Deprecated                        |                                                                                                                                                                     |

## Specialized Services

| Service                               | Description                                           |
|---------------------------------------|-------------------------------------------------------|
| [Logger](./logger/index.md)           | Writes a notification to a configured Go `log.Logger` |
| [Generic Webhook](./generic/index.md) | Sends notifications directly to a webhook             |
