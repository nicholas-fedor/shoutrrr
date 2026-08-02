package ntfy

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("apiResponseError", func() {
	ginkgo.Describe("Error", func() {
		ginkgo.It("should format message and code", func() {
			e := &apiResponseError{
				Code:    404,
				Message: "not found",
			}
			gomega.Expect(e.Error()).To(gomega.Equal("server response: not found (404)"))
		})

		ginkgo.It("should include link when present", func() {
			e := &apiResponseError{
				Code:    400,
				Message: "bad request",
				Link:    "https://docs.ntfy.sh",
			}
			gomega.Expect(e.Error()).To(gomega.Equal(
				"server response: bad request (400), see: https://docs.ntfy.sh",
			))
		})

		ginkgo.It("should not include link when empty", func() {
			e := &apiResponseError{
				Code:    500,
				Message: "server error",
				Link:    "",
			}
			gomega.Expect(e.Error()).To(gomega.Equal("server response: server error (500)"))
		})

		ginkgo.It("should handle zero code", func() {
			e := &apiResponseError{
				Code:    0,
				Message: "unknown error",
			}
			gomega.Expect(e.Error()).To(gomega.Equal("server response: unknown error (0)"))
		})

		ginkgo.It("should handle empty message", func() {
			e := &apiResponseError{
				Code:    500,
				Message: "",
			}
			gomega.Expect(e.Error()).To(gomega.Equal("server response:  (500)"))
		})
	})
})
