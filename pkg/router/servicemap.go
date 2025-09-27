package router

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/googlechat"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/lark"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/mattermost"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/rocketchat"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/signal"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/slack"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/teams"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/telegram"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/wecom"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/bark"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/gotify"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ifttt"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/join"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/pushbullet"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/pushover"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/incident/opsgenie"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/specialized/generic"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/specialized/logger"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var serviceMap = map[string]func() types.Service{
	"bark":       func() types.Service { return &bark.Service{} },
	"discord":    func() types.Service { return &discord.Service{} },
	"generic":    func() types.Service { return &generic.Service{} },
	"gotify":     func() types.Service { return &gotify.Service{} },
	"googlechat": func() types.Service { return &googlechat.Service{} },
	"hangouts":   func() types.Service { return &googlechat.Service{} },
	"ifttt":      func() types.Service { return &ifttt.Service{} },
	"lark":       func() types.Service { return &lark.Service{} },
	"join":       func() types.Service { return &join.Service{} },
	"logger":     func() types.Service { return &logger.Service{} },
	"matrix":     func() types.Service { return &matrix.Service{} },
	"mattermost": func() types.Service { return &mattermost.Service{} },
	"ntfy":       func() types.Service { return &ntfy.Service{} },
	"opsgenie":   func() types.Service { return &opsgenie.Service{} },
	"pushbullet": func() types.Service { return &pushbullet.Service{} },
	"pushover":   func() types.Service { return &pushover.Service{} },
	"rocketchat": func() types.Service { return &rocketchat.Service{} },
	"signal":     func() types.Service { return &signal.Service{} },
	"slack":      func() types.Service { return &slack.Service{} },
	"smtp":       func() types.Service { return &smtp.Service{} },
	"teams":      func() types.Service { return &teams.Service{} },
	"telegram":   func() types.Service { return &telegram.Service{} },
	"wecom":      func() types.Service { return &wecom.Service{} },
	"zulip":      func() types.Service { return &zulip.Service{} },
}
