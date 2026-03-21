package matrix

import (
	"net/url"

	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.Describe("Config", func() {
	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should generate URL with scheme and host", func() {
			cfg := &Config{
				Host:     "matrix.example.com",
				User:     "testuser",
				Password: "testpass",
			}

			resultURL := cfg.GetURL()
			gomega.Expect(resultURL.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(resultURL.Host).To(gomega.Equal("matrix.example.com"))
		})

		ginkgo.It("should include user in URL", func() {
			cfg := &Config{
				Host:     "matrix.example.com",
				User:     "testuser",
				Password: "testpass",
			}

			resultURL := cfg.GetURL()
			gomega.Expect(resultURL.User.Username()).To(gomega.Equal("testuser"))
		})

		ginkgo.It("should include password in URL", func() {
			cfg := &Config{
				Host:     "matrix.example.com",
				User:     "testuser",
				Password: "testpass",
			}

			resultURL := cfg.GetURL()
			password, ok := resultURL.User.Password()
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(password).To(gomega.Equal("testpass"))
		})

		ginkgo.It("should set ForceQuery to true", func() {
			cfg := &Config{
				Host:     "matrix.example.com",
				User:     "testuser",
				Password: "testpass",
			}

			resultURL := cfg.GetURL()
			gomega.Expect(resultURL.ForceQuery).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("SetURL", func() {
		ginkgo.It("should parse valid URL and set fields", func() {
			cfg := &Config{}
			testURL, err := url.Parse("matrix://testuser:testpass@matrix.example.com?disableTLS=true")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = cfg.SetURL(testURL)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(cfg.Host).To(gomega.Equal("matrix.example.com"))
			gomega.Expect(cfg.User).To(gomega.Equal("testuser"))
			gomega.Expect(cfg.Password).To(gomega.Equal("testpass"))
		})

		ginkgo.It("should return error when host is missing", func() {
			cfg := &Config{}
			testURL, err := url.Parse("matrix://testuser:testpass@")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = cfg.SetURL(testURL)
			gomega.Expect(err).To(gomega.Equal(ErrMissingHost))
		})

		ginkgo.It("should return error when password is missing", func() {
			cfg := &Config{}
			testURL, err := url.Parse("matrix://testuser@matrix.example.com")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = cfg.SetURL(testURL)
			gomega.Expect(err).To(gomega.Equal(ErrMissingCredentials))
		})

		ginkgo.It("should prepend # to room aliases that don't have prefix", func() {
			cfg := &Config{}
			testURL, err := url.Parse("matrix://testuser:testpass@matrix.example.com?room=testroom")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = cfg.SetURL(testURL)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(cfg.Rooms).ToNot(gomega.BeEmpty())
			gomega.Expect(cfg.Rooms[0]).To(gomega.Equal("#testroom"))
		})

		ginkgo.It("should not prepend # to room aliases that already have ! prefix", func() {
			cfg := &Config{}
			testURL, err := url.Parse("matrix://testuser:testpass@matrix.example.com?room=!testroom:matrix.example.com")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = cfg.SetURL(testURL)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(cfg.Rooms[0]).To(gomega.Equal("!testroom:matrix.example.com"))
		})
	})
})
