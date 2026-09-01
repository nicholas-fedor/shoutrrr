package smtp

import (
	"errors"

	"github.com/stretchr/testify/mock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	failuresmocks "github.com/nicholas-fedor/shoutrrr/internal/failures/mocks"
)

var _ = ginkgo.Describe("fail", func() {
	ginkgo.It("should wrap the underlying error with the selected ID", func() {
		underlying := failuresmocks.NewMockFailure(ginkgo.GinkgoT())
		underlying.On("Error").Return("boom")

		got := fail(FailHandshake, underlying)
		gomega.Expect(got.ID()).To(gomega.Equal(FailHandshake))
		gomega.Expect(got.Unwrap()).To(gomega.Equal(underlying))
		gomega.Expect(got.Error()).To(gomega.ContainSubstring("server did not accept the handshake"))
		gomega.Expect(got.Error()).To(gomega.ContainSubstring("boom"))
	})

	ginkgo.It("should interpolate format verbs from extra arguments", func() {
		got := fail(FailAuthType, nil, "bogus")
		gomega.Expect(got.ID()).To(gomega.Equal(FailAuthType))
		gomega.Expect(got.Error()).To(gomega.Equal("invalid authentication method 'bogus'"))
	})

	ginkgo.It("should interpolate the recipient into FailSendRecipient", func() {
		got := fail(FailSendRecipient, errors.New("nope"), "rec@example.com")
		gomega.Expect(got.ID()).To(gomega.Equal(FailSendRecipient))
		gomega.Expect(got.Error()).To(gomega.ContainSubstring(`error sending message to recipient "rec@example.com"`))
	})

	ginkgo.It("should use the unknown message for an unrecognized ID", func() {
		got := fail(failures.FailureID(999), nil)
		gomega.Expect(got.ID()).To(gomega.Equal(failures.FailureID(999)))
		gomega.Expect(got.Error()).To(gomega.Equal("an unknown error occurred"))
	})

	ginkgo.DescribeTable("should select a descriptive message for each failure ID",
		func(id failures.FailureID, substring string) {
			got := fail(id, nil)
			gomega.Expect(got.ID()).To(gomega.Equal(id))
			gomega.Expect(got.Error()).To(gomega.ContainSubstring(substring))
		},
		ginkgo.Entry("FailUnknown", FailUnknown, "an unknown error occurred"),
		ginkgo.Entry("FailGetSMTPClient", FailGetSMTPClient, "error getting SMTP client"),
		ginkgo.Entry("FailConnectToServer", FailConnectToServer, "error connecting to server"),
		ginkgo.Entry("FailCreateSMTPClient", FailCreateSMTPClient, "error creating smtp client"),
		ginkgo.Entry("FailEnableStartTLS", FailEnableStartTLS, "error enabling StartTLS"),
		ginkgo.Entry("FailAuthenticating", FailAuthenticating, "error authenticating"),
		ginkgo.Entry("FailClosingSession", FailClosingSession, "error closing session"),
		ginkgo.Entry("FailPlainHeader", FailPlainHeader, "error writing plain header"),
		ginkgo.Entry("FailHTMLHeader", FailHTMLHeader, "error writing HTML header"),
		ginkgo.Entry("FailMultiEndHeader", FailMultiEndHeader, "error writing multipart end header"),
		ginkgo.Entry("FailMessageTemplate", FailMessageTemplate, "error applying message template"),
		ginkgo.Entry("FailMessageRaw", FailMessageRaw, "error writing message"),
		ginkgo.Entry("FailSetSender", FailSetSender, "error setting MAIL FROM"),
		ginkgo.Entry("FailSetRecipient", FailSetRecipient, "error setting RCPT"),
		ginkgo.Entry("FailOpenDataStream", FailOpenDataStream, "error starting DATA"),
		ginkgo.Entry("FailWriteHeaders", FailWriteHeaders, "error writing message headers"),
		ginkgo.Entry("FailCloseDataStream", FailCloseDataStream, "error closing message stream"),
		ginkgo.Entry("FailApplySendParams", FailApplySendParams, "error applying params to send config"),
		ginkgo.Entry("FailHandshake", FailHandshake, "server did not accept the handshake"),
		ginkgo.Entry("FailResetSession", FailResetSession, "error resetting session between recipients"),
	)

	ginkgo.It("should compare equal by ID via errors.Is", func() {
		gomega.Expect(fail(FailHandshake, errors.New("one"))).To(gomega.MatchError(fail(FailHandshake, nil)))
		gomega.Expect(fail(FailHandshake, nil)).NotTo(gomega.MatchError(fail(FailAuthenticating, nil)))
	})

	ginkgo.It("should not require the mock Error method when only unwrapping", func() {
		underlying := failuresmocks.NewMockFailure(ginkgo.GinkgoT())
		underlying.On("Error").Return("unused").Maybe()
		underlying.On("ID").Return(FailUnknown).Maybe()
		underlying.On("Unwrap").Return(nil).Maybe()
		underlying.On("Is", mock.Anything).Return(false).Maybe()

		got := fail(FailConnectToServer, underlying)
		gomega.Expect(got.Unwrap()).To(gomega.Equal(underlying))
	})
})
