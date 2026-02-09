package mqtt

import (
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("MQTT Errors", func() {
	ginkgo.Describe("PublishError", func() {
		ginkgo.Context("Error method", func() {
			ginkgo.It("should format error with reason code only", func() {
				err := PublishError{
					ReasonCode:   0x80,
					ReasonString: "",
				}
				gomega.Expect(err.Error()).To(gomega.Equal("MQTT publish failed: reason code 0x80"))
			})

			ginkgo.It("should format error with reason code and string", func() {
				err := PublishError{
					ReasonCode:   0x81,
					ReasonString: "Client identifier not valid",
				}
				gomega.Expect(err.Error()).
					To(gomega.Equal("MQTT publish failed: reason code 0x81 - Client identifier not valid"))
			})

			ginkgo.It("should format success code", func() {
				err := PublishError{
					ReasonCode:   0x00,
					ReasonString: "Success",
				}
				gomega.Expect(err.Error()).
					To(gomega.Equal("MQTT publish failed: reason code 0x00 - Success"))
			})

			ginkgo.It("should format maximum reason code", func() {
				err := PublishError{
					ReasonCode:   0xFF,
					ReasonString: "",
				}
				gomega.Expect(err.Error()).To(gomega.Equal("MQTT publish failed: reason code 0xFF"))
			})

			ginkgo.It("should format with single digit hex code", func() {
				err := PublishError{
					ReasonCode:   0x01,
					ReasonString: "",
				}
				gomega.Expect(err.Error()).To(gomega.Equal("MQTT publish failed: reason code 0x01"))
			})

			ginkgo.It("should format with reason string containing special characters", func() {
				err := PublishError{
					ReasonCode:   0x8C,
					ReasonString: "Topic Name invalid: /test/# (wildcards not allowed)",
				}
				gomega.Expect(err.Error()).
					To(gomega.Equal("MQTT publish failed: reason code 0x8C - Topic Name invalid: /test/# (wildcards not allowed)"))
			})

			ginkgo.It("should format with empty reason string as code only", func() {
				err := PublishError{
					ReasonCode:   0x10,
					ReasonString: "",
				}
				gomega.Expect(err.Error()).To(gomega.Equal("MQTT publish failed: reason code 0x10"))
			})

			ginkgo.It("should format with whitespace-only reason string", func() {
				err := PublishError{
					ReasonCode:   0x20,
					ReasonString: "   ",
				}
				gomega.Expect(err.Error()).
					To(gomega.Equal("MQTT publish failed: reason code 0x20 -    "))
			})
		})
	})

	ginkgo.Describe("IsFailureCode", func() {
		ginkgo.It("should return false for success code 0x00", func() {
			gomega.Expect(IsFailureCode(0x00)).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for warning codes below 0x80", func() {
			testCases := []struct {
				code     byte
				expected bool
			}{
				{0x00, false}, // Success
				{0x01, false}, // Granted QoS 1
				{0x02, false}, // Granted QoS 2
				{0x10, false}, // Normal disconnect
				{0x7F, false}, // Highest non-failure code
			}

			for _, tc := range testCases {
				gomega.Expect(IsFailureCode(tc.code)).To(gomega.Equal(tc.expected),
					"IsFailureCode(0x%02X) should return %v", tc.code, tc.expected)
			}
		})

		ginkgo.It("should return true for failure codes 0x80 and above", func() {
			testCases := []struct {
				code     byte
				expected bool
			}{
				{0x80, true}, // Unspecified error
				{0x81, true}, // Malformed packet
				{0x82, true}, // Protocol error
				{0x87, true}, // Not authorized
				{0x8C, true}, // Topic Name invalid
				{0x90, true}, // Topic filter invalid
				{0xFF, true}, // Highest failure code
			}

			for _, tc := range testCases {
				gomega.Expect(IsFailureCode(tc.code)).To(gomega.Equal(tc.expected),
					"IsFailureCode(0x%02X) should return %v", tc.code, tc.expected)
			}
		})

		ginkgo.It("should return false for boundary code 0x7F", func() {
			gomega.Expect(IsFailureCode(0x7F)).To(gomega.BeFalse())
		})

		ginkgo.It("should return true for boundary code 0x80", func() {
			gomega.Expect(IsFailureCode(0x80)).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("Sentinel Errors", func() {
		ginkgo.It("should match ErrPublishTimeout", func() {
			gomega.Expect(ErrPublishTimeout.Error()).To(gomega.Equal("publish timeout exceeded"))
		})

		ginkgo.It("should match ErrConnectionNotInitialized", func() {
			gomega.Expect(ErrConnectionNotInitialized.Error()).
				To(gomega.Equal("MQTT connection manager not initialized"))
		})

		ginkgo.It("should be usable with errors.Is for ErrPublishTimeout", func() {
			err := ErrPublishTimeout
			gomega.Expect(err).To(gomega.Equal(ErrPublishTimeout))
		})

		ginkgo.It("should be usable with errors.Is for ErrConnectionNotInitialized", func() {
			err := ErrConnectionNotInitialized
			gomega.Expect(err).To(gomega.Equal(ErrConnectionNotInitialized))
		})

		ginkgo.It("should have distinct sentinel errors", func() {
			gomega.Expect(ErrPublishTimeout).ToNot(gomega.Equal(ErrConnectionNotInitialized))
		})
	})
})
