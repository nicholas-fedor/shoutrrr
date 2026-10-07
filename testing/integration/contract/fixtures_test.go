package contract_test

import "net/http"

// netKind is how a service reaches the network.
type netKind int

// fixture is a valid service URL for one scheme. Every value in secrets appears in
// url and must never show up in errors or logs.
type fixture struct {
	scheme  string
	url     string
	kind    netKind
	secrets []string
	status  int    // response status for the injected client; defaults to 200
	body    string // response body for the injected client; defaults to "{}"
}

const (
	netHTTP netKind = iota // uses the injected types.HTTPClient
	netTCP                 // uses the injected DialContext
	netNone                // performs no network I/O
)

// fixtures covers every scheme returned by router.SupportedSchemas.
// Configurable hosts use the reserved .invalid TLD.
var fixtures = []fixture{
	{
		scheme:  "bark",
		url:     "bark://:SECRETbarkDEVICEKEY@bark.example.invalid",
		secrets: []string{"SECRETbarkDEVICEKEY"},
		body:    `{"code":200,"message":"success"}`,
	},
	{
		scheme:  "discord",
		url:     "discord://SECRETdiscordTOKEN@123456789012345678",
		secrets: []string{"SECRETdiscordTOKEN"},
		status:  http.StatusNoContent,
	},
	{
		scheme:  "generic",
		url:     "generic://hooks.example.invalid/webhook?@Authorization=Bearer%20SECRETgenericHEADER&token=SECRETgenericQUERY",
		secrets: []string{"SECRETgenericHEADER", "SECRETgenericQUERY"},
	},
	{
		scheme:  "googlechat",
		url:     "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=SECRETgchatKEY&token=SECRETgchatTOKEN",
		secrets: []string{"SECRETgchatKEY", "SECRETgchatTOKEN"},
	},
	{
		scheme:  "gotify",
		url:     "gotify://gotify.example.invalid/ASECRETgotifyTK",
		secrets: []string{"ASECRETgotifyTK"},
		body:    `{"id":1}`,
	},
	{
		scheme:  "hangouts",
		url:     "hangouts://chat.googleapis.com/v1/spaces/FOO/messages?key=SECREThangoutsKEY&token=SECREThangoutsTOKEN",
		secrets: []string{"SECREThangoutsKEY", "SECREThangoutsTOKEN"},
	},
	{
		scheme:  "homeassistant",
		url:     "homeassistant://SECREThassTOKEN@ha.example.invalid",
		secrets: []string{"SECREThassTOKEN"},
		body:    `[]`,
	},
	{
		scheme:  "ifttt",
		url:     "ifttt://SECRETiftttKEY/?events=event",
		secrets: []string{"SECRETiftttKEY"},
		status:  http.StatusNoContent,
	},
	{
		scheme:  "join",
		url:     "join://:SECRETjoinAPIKEY@join/?devices=device",
		secrets: []string{"SECRETjoinAPIKEY"},
	},
	{
		scheme:  "lark",
		url:     "lark://open.larksuite.com/SECRETlarkTOKEN?secret=SECRETlarkSIGN",
		secrets: []string{"SECRETlarkTOKEN", "SECRETlarkSIGN"},
		body:    `{"code":0,"msg":"success"}`,
	},
	{
		scheme:  "logger",
		url:     "logger://",
		kind:    netNone,
		secrets: nil,
	},
	{
		scheme:  "matrix",
		url:     "matrix://contract:SECRETmatrixPASSWORD@matrix.example.invalid",
		secrets: []string{"SECRETmatrixPASSWORD"},
	},
	{
		scheme:  "mattermost",
		url:     "mattermost://contract@mattermost.example.invalid/SECRETmattermostTOKEN",
		secrets: []string{"SECRETmattermostTOKEN"},
	},
	{
		scheme:  "mqtt",
		url:     "mqtt://contract:SECRETmqttPASSWORD@broker.example.invalid:1883/contract/topic",
		kind:    netTCP,
		secrets: []string{"SECRETmqttPASSWORD"},
	},
	{
		scheme:  "mqtts",
		url:     "mqtts://contract:SECRETmqttsPASSWORD@broker.example.invalid:8883/contract/topic",
		kind:    netTCP,
		secrets: []string{"SECRETmqttsPASSWORD"},
	},
	{
		scheme:  "notifiarr",
		url:     "notifiarr://SECRETnotifiarrAPIKEY",
		secrets: []string{"SECRETnotifiarrAPIKEY"},
	},
	{
		scheme:  "ntfy",
		url:     "ntfy://contract:SECRETntfyPASSWORD@ntfy.example.invalid/topic",
		secrets: []string{"SECRETntfyPASSWORD"},
	},
	{
		scheme:  "opsgenie",
		url:     "opsgenie://opsgenie.example.invalid/SECRETopsgenieKEY?responders=user:dummy",
		secrets: []string{"SECRETopsgenieKEY"},
	},
	{
		scheme:  "pagerduty",
		url:     "pagerduty://events.example.invalid/5ec7e75ec7e75ec7e75ec7e75ec7e700",
		secrets: []string{"5ec7e75ec7e75ec7e75ec7e75ec7e700"},
		status:  http.StatusAccepted,
	},
	{
		scheme:  "pushbullet",
		url:     "pushbullet://SECRETpushbulletTOKENxxxxxxxxxxxxx",
		secrets: []string{"SECRETpushbulletTOKENxxxxxxxxxxxxx"},
		body:    `{"type":"note","body":"contract","title":"contract","active":true,"created":0}`,
	},
	{
		scheme:  "pushover",
		url:     "pushover://:SECRETpushoverTOKEN@SECRETpushoverUSER/?devices=device",
		secrets: []string{"SECRETpushoverTOKEN", "SECRETpushoverUSER"},
	},
	{
		scheme:  "rocketchat",
		url:     "rocketchat://rocketchat.example.invalid/SECRETrocketTOKEN/channel",
		secrets: []string{"SECRETrocketTOKEN"},
	},
	{
		scheme:  "signal",
		url:     "signal://signal.example.invalid:8080/+15551234567/+15559876543?token=SECRETsignalTOKEN",
		secrets: []string{"SECRETsignalTOKEN"},
		body:    `{"timestamp":1}`,
	},
	{
		scheme:  "signalgrid",
		url:     "signalgrid://SECRETsignalgridKEY@channel",
		secrets: []string{"SECRETsignalgridKEY"},
	},
	{
		scheme:  "slack",
		url:     "slack://AAAAAAAAA/BBBBBBBBB/SECRETslackTOKENxxxxxxxx",
		secrets: []string{"SECRETslackTOKENxxxxxxxx"},
	},
	{
		scheme:  "smtp",
		url:     "smtp://contract:SECRETsmtpPASSWORD@mail.example.invalid:587/?fromAddress=from@example.invalid&toAddresses=to@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETsmtpPASSWORD"},
	},
	{
		scheme: "teams",
		url: "teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2F00000000000000000000000000000000" +
			"%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-01%26sp%3D%252Ftriggers%252Fmanual%252Frun" +
			"%26sv%3D1.0%26sig%3DSECRETteamsSIG",
		secrets: []string{"SECRETteamsSIG"},
	},
	{
		scheme:  "telegram",
		url:     "telegram://123456789:SECRETtelegramTOKENxxxxxxxxxxxxxxxx@telegram?chats=contract",
		secrets: []string{"SECRETtelegramTOKENxxxxxxxxxxxxxxxx"},
		body:    `{"ok":true,"result":{"message_id":1}}`,
	},
	{
		scheme:  "twilio",
		url:     "twilio://ACaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:SECRETtwilioTOKEN@+15551234567/+15559876543",
		secrets: []string{"SECRETtwilioTOKEN"},
		status:  http.StatusCreated,
		body:    `{"sid":"SM123"}`,
	},
	{
		scheme:  "wecom",
		url:     "wecom://SECRETwecomKEY",
		secrets: []string{"SECRETwecomKEY"},
		body:    `{"errcode":0,"errmsg":"ok"}`,
	},
	{
		scheme:  "xmpp",
		url:     "xmpp://contract:SECRETxmppPASSWORD@xmpp.example.invalid/?to=bob@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETxmppPASSWORD"},
	},
	{
		scheme:  "xmpps",
		url:     "xmpps://contract:SECRETxmppsPASSWORD@xmpp.example.invalid/?to=bob@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETxmppsPASSWORD"},
	},
	{
		scheme:  "zulip",
		url:     "zulip://bot%40example.invalid:SECRETzulipKEY@zulip.example.invalid/?stream=foo&topic=bar",
		secrets: []string{"SECRETzulipKEY"},
	},
}

// allowlist records known contract failures by check and scheme, each tagged with
// its audit finding. Fixing a failure must remove its entry; TestContract fails if
// an allowlisted case passes.
var allowlist = map[string]map[string]knownFailure{
	checkInitializeNoIO: {
		"matrix": {"F3: Matrix logs in during Initialize", failInitIO},
	},
	checkSendUsesInjection: {
		"matrix": {"F3: Matrix logs in during Initialize", failInit},
	},
	checkReservedParams: {
		"bark":          {"F9: rejects reserved params", failRejectedParams},
		"discord":       {"F9: rejects reserved params", failRejectedParams},
		"gotify":        {"F9: rejects reserved params", failRejectedParams},
		"homeassistant": {"F9: rejects reserved params", failRejectedParams},
		"ifttt":         {"F9: rejects reserved params", failRejectedParams},
		"lark":          {"F9: rejects reserved params", failRejectedParams},
		"matrix":        {"F3: Matrix logs in during Initialize", failInit},
		"mattermost":    {"F9: rejects reserved params", failRejectedParams},
		"mqtt":          {"F9: rejects reserved params", failRejectedParams},
		"mqtts":         {"F9: rejects reserved params", failRejectedParams},
		"ntfy":          {"F9: rejects reserved params", failRejectedParams},
		"opsgenie":      {"F9: rejects reserved params", failRejectedParams},
		"pagerduty":     {"F9: rejects reserved params", failRejectedParams},
		"pushbullet":    {"F9: rejects reserved params", failRejectedParams},
		"pushover":      {"F9: rejects reserved params", failRejectedParams},
		"signal":        {"F9: rejects reserved params", failRejectedParams},
		"signalgrid":    {"F9: rejects reserved params", failRejectedParams},
		"slack":         {"F9: rejects reserved params", failRejectedParams},
		"smtp":          {"F9: rejects reserved params", failRejectedParams},
		"teams":         {"F9: rejects reserved params", failRejectedParams},
		"telegram":      {"F9: rejects reserved params", failRejectedParams},
		"twilio":        {"F9: rejects reserved params", failRejectedParams},
		"wecom":         {"F9: rejects reserved params", failRejectedParams},
		"xmpp":          {"F9: rejects reserved params", failRejectedParams},
		"xmpps":         {"F9: rejects reserved params", failRejectedParams},
		"zulip":         {"F9: rejects reserved params", failRejectedParams},
	},
	checkNilHTTPClient: {
		"discord":    {"F7: panics after SetHTTPClient(nil)", failPanic},
		"googlechat": {"F7: panics after SetHTTPClient(nil)", failPanic},
		"hangouts":   {"F7: panics after SetHTTPClient(nil)", failPanic},
		"matrix":     {"F3: Matrix logs in during Initialize", failInit},
		"ntfy":       {"F7: keeps the injected client after SetHTTPClient(nil)", failInjectedAfterNil},
		"pushbullet": {"F7: panics after SetHTTPClient(nil)", failPanic},
		"zulip":      {"F7: panics after SetHTTPClient(nil)", failPanic},
	},
	checkNoSecretLeaks: {
		"discord":    {"F1: leaks secrets in errors or logs", failSecretLeak},
		"generic":    {"F1: leaks secrets in errors or logs", failSecretLeak},
		"googlechat": {"F1: leaks secrets in errors or logs", failSecretLeak},
		"gotify":     {"F1: leaks secrets in errors or logs", failSecretLeak},
		"hangouts":   {"F1: leaks secrets in errors or logs", failSecretLeak},
		"ifttt":      {"F1: leaks secrets in errors or logs", failSecretLeak},
		"join":       {"F1: leaks secrets in errors or logs", failSecretLeak},
		"lark":       {"F1: leaks secrets in errors or logs", failSecretLeak},
		"mattermost": {"F1: leaks secrets in errors or logs", failSecretLeak},
		"notifiarr":  {"F1: leaks secrets in errors or logs", failSecretLeak},
		"rocketchat": {"F1: leaks secrets in errors or logs", failSecretLeak},
		"slack":      {"F1: leaks secrets in errors or logs", failSecretLeak},
		"teams":      {"F1: leaks secrets in errors or logs", failSecretLeak},
		"wecom":      {"F1: leaks secrets in errors or logs", failSecretLeak},
		"zulip":      {"F1: leaks secrets in errors or logs", failSecretLeak},
	},
}
