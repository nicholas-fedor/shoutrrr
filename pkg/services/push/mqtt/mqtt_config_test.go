package mqtt

import (
	"net/url"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Config", func() {
	ginkgo.Context("GetURL", func() {
		ginkgo.It("should build URL with all fields populated", func() {
			config := &Config{
				Host:                   "broker.example.com",
				Port:                   1883,
				Topic:                  "notifications/alerts",
				Username:               "testuser",
				Password:               "testpass",
				ClientID:               "shoutrrr-client",
				QoS:                    QoSAtLeastOnce,
				Retained:               true,
				CleanSession:           false,
				DisableTLS:             true,
				DisableTLSVerification: true,
			}

			result := config.GetURL()

			gomega.Expect(result.Scheme).To(gomega.Equal("mqtt"))
			gomega.Expect(result.Hostname()).To(gomega.Equal("broker.example.com"))
			gomega.Expect(result.Port()).To(gomega.Equal("1883"))
			gomega.Expect(result.Path).To(gomega.Equal("notifications/alerts"))
			gomega.Expect(result.User.Username()).To(gomega.Equal("testuser"))
			pass, hasPass := result.User.Password()
			gomega.Expect(hasPass).To(gomega.BeTrue())
			gomega.Expect(pass).To(gomega.Equal("testpass"))
			gomega.Expect(result.Query().Get("clientid")).To(gomega.Equal("shoutrrr-client"))
			gomega.Expect(result.Query().Get("qos")).To(gomega.Equal("AtLeastOnce"))
			gomega.Expect(result.Query().Get("retained")).To(gomega.Equal("Yes"))
			gomega.Expect(result.Query().Get("cleansession")).To(gomega.Equal("No"))
			gomega.Expect(result.Query().Get("disabletls")).To(gomega.Equal("Yes"))
			gomega.Expect(result.Query().Get("disabletlsverification")).To(gomega.Equal("Yes"))
		})

		ginkgo.It("should build URL without credentials", func() {
			config := &Config{
				Host:     "broker.example.com",
				Port:     1883,
				Topic:    "notifications/alerts",
				ClientID: "shoutrrr",
				QoS:      QoSAtMostOnce,
			}

			result := config.GetURL()

			gomega.Expect(result.User).To(gomega.BeNil())
			gomega.Expect(result.Hostname()).To(gomega.Equal("broker.example.com"))
			gomega.Expect(result.Path).To(gomega.Equal("notifications/alerts"))
		})

		ginkgo.It("should build URL with username only", func() {
			config := &Config{
				Host:     "broker.example.com",
				Port:     1883,
				Topic:    "notifications/alerts",
				Username: "testuser",
				ClientID: "shoutrrr",
				QoS:      QoSAtMostOnce,
			}

			result := config.GetURL()

			gomega.Expect(result.User.Username()).To(gomega.Equal("testuser"))
			_, hasPass := result.User.Password()
			gomega.Expect(hasPass).To(gomega.BeFalse())
		})

		ginkgo.It("should build URL with default ports", func() {
			config := &Config{
				Host:     "broker.example.com",
				Port:     1883,
				Topic:    "test/topic",
				ClientID: "shoutrrr",
				QoS:      QoSAtMostOnce,
			}

			result := config.GetURL()

			gomega.Expect(result.Port()).To(gomega.Equal("1883"))
		})

		ginkgo.It("should handle special characters in topic", func() {
			config := &Config{
				Host:     "broker.example.com",
				Port:     1883,
				Topic:    "notifications/alerts/critical",
				ClientID: "shoutrrr",
				QoS:      QoSAtMostOnce,
			}

			result := config.GetURL()

			gomega.Expect(result.Path).To(gomega.Equal("notifications/alerts/critical"))
		})

		ginkgo.It("should build URL with QoS exactly once", func() {
			config := &Config{
				Host:     "broker.example.com",
				Port:     1883,
				Topic:    "test/topic",
				ClientID: "shoutrrr",
				QoS:      QoSExactlyOnce,
			}

			result := config.GetURL()

			gomega.Expect(result.Query().Get("qos")).To(gomega.Equal("ExactlyOnce"))
		})
	})

	ginkgo.Context("SetURL", func() {
		ginkgo.It("should parse complete URL with all components", func() {
			config := &Config{}
			testURL, err := url.Parse(
				"mqtt://testuser:testpass@broker.example.com:1883/notifications/alerts?clientid=myclient&qos=1&retained=yes&cleansession=no&disabletls=yes&disabletlsverification=yes",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Host).To(gomega.Equal("broker.example.com"))
			gomega.Expect(config.Port).To(gomega.Equal(1883))
			gomega.Expect(config.Topic).To(gomega.Equal("notifications/alerts"))
			gomega.Expect(config.Username).To(gomega.Equal("testuser"))
			gomega.Expect(config.Password).To(gomega.Equal("testpass"))
			gomega.Expect(config.ClientID).To(gomega.Equal("myclient"))
			gomega.Expect(config.QoS).To(gomega.Equal(QoSAtLeastOnce))
			gomega.Expect(config.Retained).To(gomega.BeTrue())
			gomega.Expect(config.CleanSession).To(gomega.BeFalse())
			gomega.Expect(config.DisableTLS).To(gomega.BeTrue())
			gomega.Expect(config.DisableTLSVerification).To(gomega.BeTrue())
		})

		ginkgo.It("should parse URL without credentials", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/notifications/alerts")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Username).To(gomega.BeEmpty())
			gomega.Expect(config.Password).To(gomega.BeEmpty())
			gomega.Expect(config.Host).To(gomega.Equal("broker.example.com"))
			gomega.Expect(config.Topic).To(gomega.Equal("notifications/alerts"))
		})

		ginkgo.It("should parse URL with query parameters for QoS and flags", func() {
			config := &Config{}
			testURL, err := url.Parse(
				"mqtt://broker.example.com:1883/test/topic?qos=2&retained=yes&cleansession=no",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.QoS).To(gomega.Equal(QoSExactlyOnce))
			gomega.Expect(config.Retained).To(gomega.BeTrue())
			gomega.Expect(config.CleanSession).To(gomega.BeFalse())
		})

		ginkgo.It("should return error for empty topic", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).To(gomega.Equal(ErrTopicRequired))
		})

		ginkgo.It("should handle default port for mqtt scheme", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Port).To(gomega.Equal(1883))
		})

		ginkgo.It("should handle default port for mqtts scheme", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtts://broker.example.com/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Port).To(gomega.Equal(8883))
		})

		ginkgo.It("should parse client_id from query", func() {
			config := &Config{}
			testURL, err := url.Parse(
				"mqtt://broker.example.com:1883/test/topic?clientid=custom-client-id",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.ClientID).To(gomega.Equal("custom-client-id"))
		})

		ginkgo.It("should parse retained flag from query", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?retained=yes")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Retained).To(gomega.BeTrue())
		})

		ginkgo.It("should parse clean_session flag from query", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?cleansession=no")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.CleanSession).To(gomega.BeFalse())
		})

		ginkgo.It("should parse disable_tls flag from query", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?disabletls=yes")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.DisableTLS).To(gomega.BeTrue())
		})

		ginkgo.It("should parse disable_tls_verification flag from query", func() {
			config := &Config{}
			testURL, err := url.Parse(
				"mqtt://broker.example.com:1883/test/topic?disabletlsverification=yes",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.DisableTLSVerification).To(gomega.BeTrue())
		})

		ginkgo.It("should return error for invalid QoS value", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?qos=5")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("not a valid enum value"))
		})

		ginkgo.It("should parse URL with username only", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://testuser@broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Username).To(gomega.Equal("testuser"))
			gomega.Expect(config.Password).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle nested topic paths", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/level1/level2/level3")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Topic).To(gomega.Equal("level1/level2/level3"))
		})

		ginkgo.It("should skip topic validation for dummy URL", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://dummy@dummy.com")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should parse QoS using numeric string", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?qos=0")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.QoS).To(gomega.Equal(QoSAtMostOnce))
		})

		ginkgo.It("should parse QoS using string name", func() {
			config := &Config{}
			testURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?qos=AtLeastOnce")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.QoS).To(gomega.Equal(QoSAtLeastOnce))
		})
	})

	ginkgo.Context("QueryFields", func() {
		ginkgo.It("should return all configurable field names", func() {
			config := &Config{}
			fields := config.QueryFields()

			gomega.Expect(fields).To(gomega.ContainElements(
				"clientid",
				"qos",
				"retained",
				"cleansession",
				"disabletls",
				"disabletlsverification",
			))
		})

		ginkgo.It("should return non-empty slice", func() {
			config := &Config{}
			fields := config.QueryFields()

			gomega.Expect(fields).NotTo(gomega.BeEmpty())
		})
	})

	ginkgo.Context("Enums", func() {
		ginkgo.It("should return QoS enum formatter", func() {
			config := &Config{}
			enums := config.Enums()

			gomega.Expect(enums).To(gomega.HaveKey("QoS"))
		})

		ginkgo.It("should return map with exactly one entry", func() {
			config := &Config{}
			enums := config.Enums()

			gomega.Expect(enums).To(gomega.HaveLen(1))
		})
	})
})

var _ = ginkgo.Describe("QoS", func() {
	ginkgo.Context("String", func() {
		ginkgo.It("should return 'AtMostOnce' for QoSAtMostOnce (0)", func() {
			qos := QoSAtMostOnce
			gomega.Expect(qos.String()).To(gomega.Equal("AtMostOnce"))
		})

		ginkgo.It("should return 'AtLeastOnce' for QoSAtLeastOnce (1)", func() {
			qos := QoSAtLeastOnce
			gomega.Expect(qos.String()).To(gomega.Equal("AtLeastOnce"))
		})

		ginkgo.It("should return 'ExactlyOnce' for QoSExactlyOnce (2)", func() {
			qos := QoSExactlyOnce
			gomega.Expect(qos.String()).To(gomega.Equal("ExactlyOnce"))
		})

		ginkgo.It("should return formatted string for invalid values", func() {
			qos := QoS(100)
			result := qos.String()
			gomega.Expect(result).NotTo(gomega.BeEmpty())
		})
	})

	ginkgo.Context("IsValid", func() {
		ginkgo.It("should return true for QoSAtMostOnce (0)", func() {
			qos := QoSAtMostOnce
			gomega.Expect(qos.IsValid()).To(gomega.BeTrue())
		})

		ginkgo.It("should return true for QoSAtLeastOnce (1)", func() {
			qos := QoSAtLeastOnce
			gomega.Expect(qos.IsValid()).To(gomega.BeTrue())
		})

		ginkgo.It("should return true for QoSExactlyOnce (2)", func() {
			qos := QoSExactlyOnce
			gomega.Expect(qos.IsValid()).To(gomega.BeTrue())
		})

		ginkgo.It("should return false for invalid value 3", func() {
			qos := QoS(3)
			gomega.Expect(qos.IsValid()).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for invalid value 255", func() {
			qos := QoS(255)
			gomega.Expect(qos.IsValid()).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for negative value", func() {
			qos := QoS(-1)
			gomega.Expect(qos.IsValid()).To(gomega.BeFalse())
		})
	})
})

var _ = ginkgo.Describe("QoSValues", func() {
	ginkgo.Context("Enum formatter", func() {
		ginkgo.It("should have correct AtMostOnce value", func() {
			gomega.Expect(QoSValues.AtMostOnce).To(gomega.Equal(QoSAtMostOnce))
		})

		ginkgo.It("should have correct AtLeastOnce value", func() {
			gomega.Expect(QoSValues.AtLeastOnce).To(gomega.Equal(QoSAtLeastOnce))
		})

		ginkgo.It("should have correct ExactlyOnce value", func() {
			gomega.Expect(QoSValues.ExactlyOnce).To(gomega.Equal(QoSExactlyOnce))
		})

		ginkgo.It("should have non-nil Enum formatter", func() {
			gomega.Expect(QoSValues.Enum).NotTo(gomega.BeNil())
		})
	})
})
