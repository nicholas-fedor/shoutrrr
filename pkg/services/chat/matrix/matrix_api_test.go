package matrix

import (
	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.Describe("API", func() {
	ginkgo.Describe("apiResError", func() {
		ginkgo.Describe("Error", func() {
			ginkgo.It("should return error message from Message field", func() {
				err := &apiResError{
					Message: "test error message",
					Code:    "M_UNKNOWN",
				}
				gomega.Expect(err.Error()).To(gomega.Equal("test error message"))
			})

			ginkgo.It("should return empty string when Message is empty", func() {
				err := &apiResError{
					Code: "M_NOT_FOUND",
				}
				gomega.Expect(err.Error()).To(gomega.Equal(""))
			})

			ginkgo.It("should return message when Code is empty but Message is set", func() {
				err := &apiResError{
					Message: "custom error",
				}
				gomega.Expect(err.Error()).To(gomega.Equal("custom error"))
			})
		})
	})

	ginkgo.Describe("newUserIdentifier", func() {
		ginkgo.It("should create identifier with user field set", func() {
			id := newUserIdentifier("testuser")
			gomega.Expect(id.User).To(gomega.Equal("testuser"))
			gomega.Expect(id.Type).To(gomega.Equal(idTypeUser))
		})

		ginkgo.It("should return empty identifier for empty user string", func() {
			id := newUserIdentifier("")
			gomega.Expect(id.User).To(gomega.BeEmpty())
			gomega.Expect(id.Type).To(gomega.Equal(idTypeUser))
		})
	})
})
