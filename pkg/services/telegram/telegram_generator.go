package telegram

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/generator"
)

// UpdatesLimit defines the number of updates to retrieve per API call.
const (
	UpdatesLimit   = 10  // Number of updates to retrieve per call
	UpdatesTimeout = 120 // Timeout in seconds for long polling
)

// ErrNoChatsSelected indicates that no chats were selected during generation.
var (
	ErrNoChatsSelected = errors.New("no chats were selected")
)

// Generator facilitates Telegram-specific URL generation via user interaction.
type Generator struct {
	userDialog *generator.UserDialog
	client     *Client
	chats      []string
	chatNames  []string
	chatTypes  []string
	done       bool
	botName    string
	Reader     io.Reader
	Writer     io.Writer
}

// Generate creates a Telegram Shoutrrr configuration from user dialog input.
func (g *Generator) Generate(
	_ types.Service,
	props map[string]string,
	_ []string,
) (types.ServiceConfig, error) {
	var config Config

	if g.Reader == nil {
		g.Reader = os.Stdin
	}

	if g.Writer == nil {
		g.Writer = os.Stdout
	}

	g.userDialog = generator.NewUserDialog(g.Reader, g.Writer, props)
	userDialog := g.userDialog

	userDialog.Writelnf(
		"To start we need your bot token. If you haven't created a bot yet, you can use this link:",
	)
	userDialog.Writelnf("  %v", format.ColorizeLink("https://t.me/botfather?start"))
	userDialog.Writelnf("")

	token := userDialog.QueryString(
		"Enter your bot token:",
		generator.ValidateFormat(IsTokenValid),
		"token",
	)

	userDialog.Writelnf("Fetching bot info...")

	g.client = &Client{token: token}

	botInfo, err := g.client.GetBotInfo()
	if err != nil {
		return &Config{}, err
	}

	g.botName = botInfo.Username

	userDialog.Writelnf("")
	userDialog.Writelnf(
		"Okay! %v will listen for any messages in PMs and group chats it is invited to.",
		format.ColorizeString("@", g.botName),
	)

	g.done = false
	lastUpdate := 0

	signals := make(chan os.Signal, 1)

	// Subscribe to system signals
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for !g.done {
		userDialog.Writelnf("Waiting for messages to arrive...")

		updates, err := g.client.GetUpdates(lastUpdate, UpdatesLimit, UpdatesTimeout, nil)
		if err != nil {
			panic(err)
		}

		// If no updates were retrieved, prompt user to continue
		promptDone := len(updates) == 0

		for _, update := range updates {
			lastUpdate = update.UpdateID + 1

			switch {
			case update.Message != nil || update.ChannelPost != nil:
				message := update.Message
				if update.ChannelPost != nil {
					message = update.ChannelPost
				}

				chat := message.Chat

				source := message.Chat.Name()
				if message.From != nil {
					source = "@" + message.From.Username
				}

				userDialog.Writelnf("Got Message '%v' from %v in %v chat %v",
					format.ColorizeString(message.Text),
					format.ColorizeProp(source),
					format.ColorizeEnum(chat.Type),
					format.ColorizeNumber(chat.ID))
				userDialog.Writelnf(g.addChat(chat))
				// Another chat was added, prompt user to continue
				promptDone = true

			case update.ChatMemberUpdate != nil:
				cmu := update.ChatMemberUpdate
				oldStatus := cmu.OldChatMember.Status
				newStatus := cmu.NewChatMember.Status
				userDialog.Writelnf(
					"Got a bot chat member update for %v, status was changed from %v to %v",
					format.ColorizeProp(cmu.Chat.Name()),
					format.ColorizeEnum(oldStatus),
					format.ColorizeEnum(newStatus),
				)

			default:
				userDialog.Writelnf("Got unknown Update. Ignored!")
			}
		}

		if promptDone {
			userDialog.Writelnf("")

			g.done = !userDialog.QueryBool(
				fmt.Sprintf("Got %v chat ID(s) so far. Want to add some more?",
					format.ColorizeNumber(len(g.chats))),
				"",
			)
		}
	}

	userDialog.Writelnf("")
	userDialog.Writelnf("Cleaning up the bot session...")

	// Notify API that we got the updates
	if _, err = g.client.GetUpdates(lastUpdate, 0, 0, nil); err != nil {
		g.userDialog.Writelnf(
			"Failed to mark last updates as received: %v",
			format.ColorizeError(err),
		)
	}

	if len(g.chats) < 1 {
		return nil, ErrNoChatsSelected
	}

	userDialog.Writelnf("Selected chats:")

	for i, id := range g.chats {
		name := g.chatNames[i]
		chatType := g.chatTypes[i]
		userDialog.Writelnf(
			"  %v (%v) %v",
			format.ColorizeNumber(id),
			format.ColorizeEnum(chatType),
			format.ColorizeString(name),
		)
	}

	userDialog.Writelnf("")

	config = Config{
		Notification: true,
		Token:        token,
		Chats:        g.chats,
	}

	return &config, nil
}

// addChat adds a chat to the generator's list if it’s not already present.
func (g *Generator) addChat(chat *Chat) string {
	chatID := strconv.FormatInt(chat.ID, 10)
	name := chat.Name()

	if slices.Contains(g.chats, chatID) {
		return fmt.Sprintf("chat %v is already selected!", format.ColorizeString(name))
	}

	g.chats = append(g.chats, chatID)
	g.chatNames = append(g.chatNames, name)
	g.chatTypes = append(g.chatTypes, chat.Type)

	return fmt.Sprintf("Added new chat %v!", format.ColorizeString(name))
}
