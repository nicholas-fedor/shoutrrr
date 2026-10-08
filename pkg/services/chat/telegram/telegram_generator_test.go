package telegram_test

import (
	"fmt"
	"io"
	"strings"

	"github.com/jarcoal/httpmock"
	"github.com/mattn/go-colorable"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/telegram"
)

const (
	mockToken   = `0:MockToken`
	mockAPIBase = "https://api.telegram.org/bot" + mockToken + "/"
)

var (
	userOut    *gbytes.Buffer
	userIn     *gbytes.Buffer
	userInMono io.Writer
)

var _ = ginkgo.Describe("TelegramGenerator", func() {
	ginkgo.BeforeEach(func() {
		userOut = gbytes.NewBuffer()
		userIn = gbytes.NewBuffer()
		userInMono = colorable.NewNonColorable(userIn)

		httpmock.Activate()
	})
	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})
	ginkgo.It("should return the ", func() {
		gen := telegram.Generator{
			Reader: userOut,
			Writer: userInMono,
		}

		resultChannel := make(chan string, 1)

		httpmock.RegisterResponder(
			"GET",
			mockAPI(`getMe`),
			httpmock.NewJsonResponderOrPanic(200, &struct {
				OK     bool
				Result *telegram.User
			}{
				true, &telegram.User{
					ID:       1,
					IsBot:    true,
					Username: "mockbot",
				},
			}),
		)

		httpmock.RegisterResponder(
			"POST",
			mockAPI(`getUpdates`),
			httpmock.NewJsonResponderOrPanic(200, &struct {
				OK     bool
				Result []telegram.Update
			}{
				true,
				[]telegram.Update{
					{
						ChatMemberUpdate: &telegram.ChatMemberUpdate{
							Chat:          &telegram.Chat{Type: `channel`, Title: `mockChannel`},
							OldChatMember: &telegram.ChatMember{Status: `kicked`},
							NewChatMember: &telegram.ChatMember{Status: `administrator`},
						},
					},
					{
						Message: &telegram.Message{
							Text: "hi!",
							From: &telegram.User{Username: `mockUser`},
							Chat: &telegram.Chat{Type: `private`, ID: 667, Username: `mockUser`},
						},
					},
				},
			}),
		)

		// Type the input before Generate starts. The dialog treats an empty buffer
		// as closed input, so typing afterwards races the first read.
		mockTyped(mockToken)
		mockTyped(`no`)

		defer dumpBuffers()

		go func() {
			defer ginkgo.GinkgoRecover()

			conf, err := gen.Generate(nil, nil, nil)

			gomega.Expect(conf).ToNot(gomega.BeNil())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resultChannel <- conf.GetURL().String()
		}()

		gomega.Eventually(userIn).
			Should(gbytes.Say(`Got a bot chat member update for mockChannel, status was changed from kicked to administrator`))
		gomega.Eventually(userIn).
			Should(gbytes.Say(`Got 1 chat ID\(s\) so far\. Want to add some more\?`))
		gomega.Eventually(userIn).Should(gbytes.Say(`Selected chats:`))
		gomega.Eventually(userIn).Should(gbytes.Say(`667 \(private\) @mockUser`))

		gomega.Eventually(resultChannel).
			Should(gomega.Receive(gomega.Equal(`telegram://0:MockToken@telegram?chats=667&preview=No`)))
	})

	ginkgo.It("should return an error when polling for updates fails", func() {
		gen := telegram.Generator{
			Reader: userOut,
			Writer: userInMono,
		}

		httpmock.RegisterResponder(
			"GET",
			mockAPI(`getMe`),
			httpmock.NewJsonResponderOrPanic(200, &struct {
				OK     bool
				Result *telegram.User
			}{
				true, &telegram.User{ID: 1, IsBot: true, Username: "mockbot"},
			}),
		)

		httpmock.RegisterResponder(
			"POST",
			mockAPI(`getUpdates`),
			httpmock.NewStringResponder(502, "<html>Bad Gateway</html>"),
		)

		errChannel := make(chan error, 1)

		// Type the input before Generate starts. The dialog treats an empty buffer
		// as closed input, so typing afterwards races the first read.
		mockTyped(mockToken)

		defer dumpBuffers()

		go func() {
			defer ginkgo.GinkgoRecover()

			_, err := gen.Generate(nil, nil, nil)
			errChannel <- err
		}()

		gomega.Eventually(errChannel).Should(gomega.Receive(gomega.MatchError(gomega.ContainSubstring("getting updates"))))
	})

	ginkgo.It("should return an error when the bot info cannot be fetched", func() {
		gen := telegram.Generator{
			Reader: userOut,
			Writer: userInMono,
		}

		httpmock.RegisterResponder(
			"GET",
			mockAPI(`getMe`),
			httpmock.NewStringResponder(502, "<html>Bad Gateway</html>"),
		)

		errChannel := make(chan error, 1)

		// Type the input before Generate starts. The dialog treats an empty buffer
		// as closed input, so typing afterwards races the first read.
		mockTyped(mockToken)

		defer dumpBuffers()

		go func() {
			defer ginkgo.GinkgoRecover()

			_, err := gen.Generate(nil, nil, nil)
			errChannel <- err
		}()

		gomega.Eventually(errChannel).Should(gomega.Receive(gomega.MatchError(gomega.ContainSubstring("getting bot info"))))
	})
})

func mockAPI(endpoint string) string {
	return mockAPIBase + endpoint
}

func mockTyped(a ...any) {
	_, _ = fmt.Fprintf(userOut, "%v\n", fmt.Sprint(a...))
}

func dumpBuffers() {
	for line := range strings.SplitSeq(string(userIn.Contents()), "\n") {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "> %s\n", line)
	}

	for line := range strings.SplitSeq(string(userOut.Contents()), "\n") {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "< %s\n", line)
	}
}
