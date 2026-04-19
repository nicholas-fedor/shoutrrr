// Dingding(i.e. DingTalk China Version) notification service.
//
// The dingding service supports both the "custombot" (自定义机器人) and "worknotice" (工作通知) types of Dingding notifications.
// The service is configured using a URL with the following format:
//
// ## Custom Bot (自定义机器人) URL format
//
//	dingding://<access_token like abcdef123...>?[kind=custombot][secret=<secret like SECabcdef123...>][&keyword=<keyword>][&title=<title>][&template=<template>]
//
// To use custombot mode, create a custom bot in any group chat in Dingding, this is yet not supported in global Dingtalk.
// You may set three types of authentication for custombot:
//
// - Custom Keywords 自定义关键词: type any keyword in the configuration page, set it in "keyword" query parameter.
// - Additional Signature 加签: copy it and set it in "secret" query parameter.
// - IP Address: use CIDR
//
// If message does not contain the keyword, it will automatically add the keyword at the end of the message content before sending to Dingding.
//
// ## Work Notice (工作通知) URL format
//
//	dingding://<client_id>?secret=<client_secret>&userids=<comma splited userIDs>&kind=worknotice[title=<title>][&template=<template>][&apiendpoint=<endpoint>]
//
// To use worknotice mode, you need to create an "Internal Application" in Dingding's developer console, and get the "AppKey" and "AppSecret" for the application.
// Add bot ablity to the application, create a bot for it and release it (both the bot and app version, this is mandatory, otherwise the API will return "robotCode not found").
// Then you can get your userID on Contact Management page.
//
// ## API endpoint
//
// By default, it use dingding's `api.dingtalk.com` endpoint. To use global dingtalk, set `apiendpoint` query parameter to `api.dingtalk.io`.
package dingding
