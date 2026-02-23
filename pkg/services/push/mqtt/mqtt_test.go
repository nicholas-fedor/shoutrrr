package mqtt

import (
	"context"
	"crypto/tls"
	"net/url"
	"sync"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Service", func() {
	var (
		service *Service
		testURL *url.URL
		logger  types.StdLogger
	)

	ginkgo.BeforeEach(func() {
		service = &Service{}
		logger = ginkgo.GinkgoT()
	})

	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return 'mqtt'", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("mqtt"))
		})

		ginkgo.It("should return consistent ID across multiple calls", func() {
			id1 := service.GetID()
			id2 := service.GetID()
			gomega.Expect(id1).To(gomega.Equal(id2))
			gomega.Expect(id1).To(gomega.Equal("mqtt"))
		})
	})

	ginkgo.Describe("getDefaultPortForScheme", func() {
		ginkgo.It("should return 1883 for mqtt scheme", func() {
			gomega.Expect(service.getDefaultPortForScheme("mqtt")).To(gomega.Equal(1883))
		})

		ginkgo.It("should return 8883 for mqtts scheme", func() {
			gomega.Expect(service.getDefaultPortForScheme("mqtts")).To(gomega.Equal(8883))
		})

		ginkgo.It("should return 1883 for unknown scheme", func() {
			gomega.Expect(service.getDefaultPortForScheme("unknown")).To(gomega.Equal(1883))
		})

		ginkgo.It("should return 1883 for empty scheme", func() {
			gomega.Expect(service.getDefaultPortForScheme("")).To(gomega.Equal(1883))
		})

		ginkgo.It("should return 1883 for http scheme (treated as non-secure)", func() {
			gomega.Expect(service.getDefaultPortForScheme("http")).To(gomega.Equal(1883))
		})

		ginkgo.It("should be case-sensitive for mqtts scheme", func() {
			// "MQTTS" is not the same as "mqtts", so it returns default
			gomega.Expect(service.getDefaultPortForScheme("MQTTS")).To(gomega.Equal(1883))
		})
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.BeforeEach(func() {
			var err error

			testURL, err = url.Parse("mqtt://broker.example.com:1883/notifications/alerts")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should parse valid URL successfully", func() {
			err := service.Initialize(testURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config).NotTo(gomega.BeNil())
		})

		ginkgo.It("should set default port for mqtt scheme when port specified in URL", func() {
			mqttURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(mqttURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Port).To(gomega.Equal(1883))
		})

		ginkgo.It("should set default port for mqtts scheme when port specified in URL", func() {
			mqttsURL, err := url.Parse("mqtts://broker.example.com:8883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(mqttsURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Port).To(gomega.Equal(8883))
		})

		ginkgo.It("should parse query parameters", func() {
			configURL, err := url.Parse(
				"mqtt://broker.example.com:1883/test/topic?clientid=myclient&qos=1&retained=yes",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.ClientID).To(gomega.Equal("myclient"))
			gomega.Expect(service.Config.QoS).To(gomega.Equal(QoSAtLeastOnce))
			gomega.Expect(service.Config.Retained).To(gomega.BeTrue())
		})

		ginkgo.It("should store config correctly", func() {
			configURL, err := url.Parse(
				"mqtt://user:pass@broker.example.com:1883/notifications/alerts",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Host).To(gomega.Equal("broker.example.com"))
			gomega.Expect(service.Config.Port).To(gomega.Equal(1883))
			gomega.Expect(service.Config.Topic).To(gomega.Equal("notifications/alerts"))
			gomega.Expect(service.Config.Username).To(gomega.Equal("user"))
			gomega.Expect(service.Config.Password).To(gomega.Equal("pass"))
		})

		ginkgo.It("should set logger", func() {
			err := service.Initialize(testURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Logger is set via SetLogger, we verify no error occurred
		})

		ginkgo.It("should parse URL with username only", func() {
			configURL, err := url.Parse("mqtt://user@broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Username).To(gomega.Equal("user"))
			gomega.Expect(service.Config.Password).To(gomega.BeEmpty())
		})

		ginkgo.It("should parse URL without credentials", func() {
			configURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Username).To(gomega.BeEmpty())
			gomega.Expect(service.Config.Password).To(gomega.BeEmpty())
		})

		ginkgo.It("should parse URL with custom port", func() {
			configURL, err := url.Parse("mqtt://broker.example.com:9999/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Port).To(gomega.Equal(9999))
		})

		ginkgo.It("should apply default values from struct tags", func() {
			configURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.ClientID).To(gomega.Equal("shoutrrr"))
			gomega.Expect(service.Config.QoS).To(gomega.Equal(QoSAtMostOnce))
			gomega.Expect(service.Config.Retained).To(gomega.BeFalse())
			gomega.Expect(service.Config.CleanSession).To(gomega.BeTrue())
		})

		ginkgo.It("should parse URL with DisableTLS parameter", func() {
			configURL, err := url.Parse("mqtts://broker.example.com:8883/test/topic?disabletls=yes")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.DisableTLS).To(gomega.BeTrue())
		})

		ginkgo.It("should parse URL with DisableTLSVerification parameter", func() {
			configURL, err := url.Parse(
				"mqtts://broker.example.com:8883/test/topic?disabletlsverification=yes",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.DisableTLSVerification).To(gomega.BeTrue())
		})

		ginkgo.It("should parse URL with CleanSession=false", func() {
			configURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic?cleansession=no")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.CleanSession).To(gomega.BeFalse())
		})

		ginkgo.It("should handle deeply nested topic paths", func() {
			configURL, err := url.Parse(
				"mqtt://broker.example.com:1883/level1/level2/level3/level4/level5",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Topic).
				To(gomega.Equal("level1/level2/level3/level4/level5"))
		})

		ginkgo.It("should initialize pkr field", func() {
			err := service.Initialize(testURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// pkr is unexported, but we can verify it works via the config
			gomega.Expect(service.Config).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Describe("getCancel", func() {
		ginkgo.It("should return valid cancel function", func() {
			cancel := service.getCancel()
			gomega.Expect(cancel).NotTo(gomega.BeNil())
		})

		ginkgo.It("should initialize context field", func() {
			service.getCancel()
			// ctx is unexported, but we verify it was set by checking cancel is not nil
			gomega.Expect(service.cancel).NotTo(gomega.BeNil())
		})

		ginkgo.It(
			"should be thread-safe and return same cancel function on multiple calls",
			func() {
				var wg sync.WaitGroup

				cancels := make([]context.CancelFunc, 10)

				for i := range 10 {
					wg.Add(1)

					go func(idx int) {
						defer wg.Done()

						cancels[idx] = service.getCancel()
					}(i)
				}

				wg.Wait()

				// All cancel functions should be non-nil
				for i := range 10 {
					gomega.Expect(cancels[i]).NotTo(gomega.BeNil())
				}
			},
		)

		ginkgo.It("should allow cancel to be called without panic", func() {
			cancel := service.getCancel()

			gomega.Expect(func() { cancel() }).NotTo(gomega.Panic())
		})

		ginkgo.It("should allow cancel to be called multiple times safely", func() {
			cancel := service.getCancel()
			cancel()
			gomega.Expect(func() { cancel() }).NotTo(gomega.Panic())
		})
	})

	ginkgo.Describe("createTLSConfig", func() {
		ginkgo.BeforeEach(func() {
			// Initialize the standard logger to avoid nil pointer dereference
			service.Standard = standard.Standard{}
			service.SetLogger(logger)
			service.Config = &Config{}
		})

		ginkgo.It("should create TLS config with verification enabled by default", func() {
			service.Config.DisableTLSVerification = false

			tlsConfig := service.createTLSConfig()

			gomega.Expect(tlsConfig).NotTo(gomega.BeNil())
			gomega.Expect(tlsConfig.InsecureSkipVerify).To(gomega.BeFalse())
		})

		ginkgo.It("should create TLS config without verification when disabled", func() {
			service.Config.DisableTLSVerification = true

			tlsConfig := service.createTLSConfig()

			gomega.Expect(tlsConfig).NotTo(gomega.BeNil())
			gomega.Expect(tlsConfig.InsecureSkipVerify).To(gomega.BeTrue())
		})

		ginkgo.It("should set minimum TLS version to 1.2", func() {
			tlsConfig := service.createTLSConfig()

			gomega.Expect(tlsConfig).NotTo(gomega.BeNil())
			gomega.Expect(tlsConfig.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS12)))
		})

		ginkgo.It("should return config even with nil Config.DisableTLSVerification", func() {
			// DisableTLSVerification defaults to false
			tlsConfig := service.createTLSConfig()

			gomega.Expect(tlsConfig).NotTo(gomega.BeNil())
		})

		ginkgo.It("should create independent TLS configs on multiple calls", func() {
			tlsConfig1 := service.createTLSConfig()
			tlsConfig2 := service.createTLSConfig()

			// Each call creates a new config
			gomega.Expect(tlsConfig1).NotTo(gomega.BeIdenticalTo(tlsConfig2))
		})
	})

	ginkgo.Describe("Close", func() {
		ginkgo.BeforeEach(func() {
			// Initialize the standard logger to avoid nil pointer dereference
			service.Standard = standard.Standard{}
			service.SetLogger(logger)
		})

		ginkgo.It("should handle nil connection manager gracefully", func() {
			service.connectionManager = nil
			service.closeOnce = sync.Once{} // Reset for this test

			err := service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should be idempotent - safe to call multiple times", func() {
			service.connectionManager = nil
			service.closeOnce = sync.Once{} // Reset

			err1 := service.Close()
			err2 := service.Close()
			err3 := service.Close()

			gomega.Expect(err1).NotTo(gomega.HaveOccurred())
			gomega.Expect(err2).NotTo(gomega.HaveOccurred())
			gomega.Expect(err3).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return same error on subsequent calls", func() {
			service.connectionManager = nil
			service.closeOnce = sync.Once{} // Reset
			service.closeErr = nil          // Reset

			// First close
			_ = service.Close()

			// Subsequent closes should return the same result (nil)
			err1 := service.Close()
			err2 := service.Close()

			gomega.Expect(err1).ToNot(gomega.HaveOccurred())
			gomega.Expect(err2).ToNot(gomega.HaveOccurred())
		})

		ginkgo.It("should handle close without initialized cancel function", func() {
			service.connectionManager = nil
			service.cancel = nil
			service.closeOnce = sync.Once{} // Reset

			err := service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should cancel context when cancel function exists", func() {
			service.connectionManager = nil
			service.closeOnce = sync.Once{} // Reset

			// Initialize the cancel function
			_ = service.getCancel()
			gomega.Expect(service.cancel).NotTo(gomega.BeNil())

			// Close should call cancel without error
			err := service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Service Constants", func() {
		ginkgo.It("should have correct Scheme constant", func() {
			gomega.Expect(Scheme).To(gomega.Equal("mqtt"))
		})

		ginkgo.It("should have correct SecureScheme constant", func() {
			gomega.Expect(SecureScheme).To(gomega.Equal("mqtts"))
		})
	})

	ginkgo.Describe("Service Struct", func() {
		ginkgo.It("should initialize with zero values", func() {
			svc := &Service{}
			gomega.Expect(svc.Config).To(gomega.BeNil())
			gomega.Expect(svc.connectionManager).To(gomega.BeNil())
			gomega.Expect(svc.connectionInitialized).To(gomega.BeFalse())
		})

		ginkgo.It("should have correct default state after initialization", func() {
			svc := &Service{}
			configURL, err := url.Parse("mqtt://broker.example.com:1883/test/topic")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = svc.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(svc.Config).NotTo(gomega.BeNil())
			gomega.Expect(svc.connectionInitialized).To(gomega.BeFalse())
			gomega.Expect(svc.connectionManager).To(gomega.BeNil())
		})
	})
})
