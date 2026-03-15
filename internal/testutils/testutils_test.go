package testutils

import (
	"net/url"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type dummyConfig struct {
	standard.EnumlessConfig

	Foo uint64 `default:"-1" key:"foo"`
}

type dummyService struct {
	standard.Standard

	Config dummyConfig
}

var _ = ginkgo.Describe("the testutils package", func() {
	ginkgo.When("calling function TestLogger", func() {
		ginkgo.It("should not return nil", func() {
			gomega.Expect(TestLogger()).NotTo(gomega.BeNil())
		})
		ginkgo.It(`should have the prefix "[Test] "`, func() {
			gomega.Expect(TestLogger().Prefix()).To(gomega.Equal("[Test] "))
		})
	})

	ginkgo.Describe("Must helpers", func() {
		ginkgo.Describe("URLMust", func() {
			ginkgo.It("should panic when an invalid URL is passed", func() {
				failures := gomega.InterceptGomegaFailures(func() { URLMust(":") })
				gomega.Expect(failures).To(gomega.HaveLen(1))
			})
		})

		ginkgo.Describe("JSONRespondMust", func() {
			ginkgo.It("should panic when an invalid struct is passed", func() {
				notAValidJSONSource := func() {}
				failures := gomega.InterceptGomegaFailures(
					func() { JSONRespondMust(200, notAValidJSONSource) },
				)
				gomega.Expect(failures).To(gomega.HaveLen(1))
			})
		})
	})

	ginkgo.Describe("Config test helpers", func() {
		var config dummyConfig

		ginkgo.BeforeEach(func() {
			config = dummyConfig{
				EnumlessConfig: standard.EnumlessConfig{},
				Foo:            0,
			}
		})
		ginkgo.Describe("TestConfigSetInvalidQueryValue", func() {
			ginkgo.It("should fail when not correctly implemented", func() {
				failures := gomega.InterceptGomegaFailures(func() {
					TestConfigSetInvalidQueryValue(&config, "mock://host?invalid=value")
				})
				gomega.Expect(failures).To(gomega.HaveLen(1))
			})
		})

		ginkgo.Describe("TestConfigGetInvalidQueryValue", func() {
			ginkgo.It("should fail when not correctly implemented", func() {
				failures := gomega.InterceptGomegaFailures(func() {
					TestConfigGetInvalidQueryValue(&config)
				})
				gomega.Expect(failures).To(gomega.HaveLen(1))
			})
		})

		ginkgo.Describe("TestConfigSetDefaultValues", func() {
			ginkgo.It("should fail when not correctly implemented", func() {
				failures := gomega.InterceptGomegaFailures(func() {
					TestConfigSetDefaultValues(&config)
				})
				gomega.Expect(failures).NotTo(gomega.BeEmpty())
			})
		})

		ginkgo.Describe("TestConfigGetEnumsCount", func() {
			ginkgo.It("should fail when not correctly implemented", func() {
				failures := gomega.InterceptGomegaFailures(func() {
					TestConfigGetEnumsCount(&config, 99)
				})
				gomega.Expect(failures).NotTo(gomega.BeEmpty())
			})
		})

		ginkgo.Describe("TestConfigGetFieldsCount", func() {
			ginkgo.It("should fail when not correctly implemented", func() {
				failures := gomega.InterceptGomegaFailures(func() {
					TestConfigGetFieldsCount(&config, 99)
				})
				gomega.Expect(failures).NotTo(gomega.BeEmpty())
			})
		})
	})

	ginkgo.Describe("Service test helpers", func() {
		var service dummyService

		ginkgo.BeforeEach(func() {
			service = dummyService{
				Standard: standard.Standard{
					Logger:    standard.Logger{},
					Templater: standard.Templater{},
				},
				Config: dummyConfig{
					EnumlessConfig: standard.EnumlessConfig{},
					Foo:            0,
				},
			}
		})
		ginkgo.Describe("TestConfigSetInvalidQueryValue", func() {
			ginkgo.It("should fail when not correctly implemented", func() {
				failures := gomega.InterceptGomegaFailures(func() {
					TestServiceSetInvalidParamValue(&service, "invalid", "value")
				})
				gomega.Expect(failures).To(gomega.HaveLen(1))
			})
		})
	})
})

func (dc *dummyConfig) Get(string) (string, error) { return "", nil }
func (dc *dummyConfig) GetURL() *url.URL {
	return &url.URL{
		Scheme:      "",
		Opaque:      "",
		User:        nil,
		Host:        "",
		Path:        "",
		RawPath:     "",
		OmitHost:    false,
		ForceQuery:  false,
		RawQuery:    "",
		Fragment:    "",
		RawFragment: "",
	}
}
func (dc *dummyConfig) QueryFields() []string    { return []string{} }
func (dc *dummyConfig) Set(string, string) error { return nil }
func (dc *dummyConfig) SetURL(_ *url.URL) error  { return nil }

func (s *dummyService) GetID() string                                  { return "dummy" }
func (s *dummyService) Initialize(_ *url.URL, _ types.StdLogger) error { return nil }
func (s *dummyService) Send(_ string, _ *types.Params) error           { return nil }

func TestTestUtils(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)

	ginkgo.RunSpecs(t, "Shoutrrr TestUtils Suite")
}
