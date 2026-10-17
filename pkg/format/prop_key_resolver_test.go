package format

import (
	"net/url"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// titledConfig is a minimal service config that declares the reserved title key.
type titledConfig struct {
	standard.EnumlessConfig

	// Title is the notification title.
	Title string `default:"" key:"title"`
}

// spelledDefaultsConfig declares defaults spelled differently from how the
// formatter prints the values.
type spelledDefaultsConfig struct {
	// Enabled is a bool whose default is spelled in lowercase.
	Enabled bool `default:"yes" key:"enabled"`
	// Quiet is a bool whose default is spelled as false.
	Quiet bool `default:"false" key:"quiet"`
	// Level is an enum whose default is spelled in lowercase.
	Level int `default:"high" key:"level"`
	// Retries is an int whose default has a sign.
	Retries int `default:"+3" key:"retries"`
	// Name is a string whose default is compared as written.
	Name string `default:"Shoutrrr" key:"name"`
	// Broken has a default that does not parse.
	Broken int `default:"NaN" key:"broken"`
	// Plain has no default.
	Plain bool `key:"plain"`
}

// spelledLevels formats the Level field of spelledDefaultsConfig.
var spelledLevels = CreateEnumFormatter([]string{"Low", "High"})

var _ = ginkgo.Describe("Prop Key Resolver", func() {
	var (
		ts  *testStruct
		pkr PropKeyResolver
	)

	ginkgo.BeforeEach(func() {
		ts = &testStruct{}
		pkr = NewPropKeyResolver(ts)
		_ = pkr.SetDefaultProps(ts)
	})
	ginkgo.Describe("Updating config props from params", func() {
		ginkgo.When("a param matches a prop key", func() {
			ginkgo.It("should be updated in the config", func() {
				err := pkr.UpdateConfigFromParams(nil, &types.Params{"str": "newValue"})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(ts.Str).To(gomega.Equal("newValue"))
			})
		})
		ginkgo.When("a param does not match a prop key", func() {
			ginkgo.It("should report the first error", func() {
				err := pkr.UpdateConfigFromParams(nil, &types.Params{"a": "z"})
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
			ginkgo.It("should process the other keys", func() {
				_ = pkr.UpdateConfigFromParams(
					nil,
					&types.Params{"signed": "1", "b": "c", "str": "val"},
				)

				gomega.Expect(ts.Signed).To(gomega.Equal(1))
				gomega.Expect(ts.Str).To(gomega.Equal("val"))
			})
		})
	})
	ginkgo.Describe("Updating config props from reserved params", func() {
		ginkgo.When("the config does not declare them", func() {
			ginkgo.It("should skip them and apply the other params", func() {
				err := pkr.UpdateConfigFromParams(nil, &types.Params{
					types.TitleKey:   "title",
					types.MessageKey: "message",
					"Level":          "warning",
					"str":            "val",
				})

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(ts.Str).To(gomega.Equal("val"))
			})
		})
		ginkgo.When("the config declares one", func() {
			ginkgo.It("should apply it", func() {
				titled := &titledConfig{}
				titledResolver := NewPropKeyResolver(titled)

				err := titledResolver.UpdateConfigFromParams(nil, &types.Params{types.TitleKey: "applied"})

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(titled.Title).To(gomega.Equal("applied"))
			})
		})
		ginkgo.When("other unknown params are present", func() {
			ginkgo.It("should still report them", func() {
				err := pkr.UpdateConfigFromParams(nil, &types.Params{types.TitleKey: "title", "titel": "typo"})

				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("titel")))
				gomega.Expect(err).To(gomega.MatchError(ErrInvalidConfigKey))
			})
		})
	})
	ginkgo.Describe("Detecting default values", func() {
		var spelled PropKeyResolver

		ginkgo.BeforeEach(func() {
			spelled = NewPropKeyResolver(&spelledDefaultsConfig{})
		})

		ginkgo.DescribeTable("should compare the value with the default as the formatter prints it",
			func(key, value string, want bool) {
				gomega.Expect(spelled.IsDefault(key, value)).To(gomega.Equal(want))
			},
			ginkgo.Entry("bool default in lowercase", "enabled", "Yes", true),
			ginkgo.Entry("bool other value", "enabled", "No", false),
			ginkgo.Entry("bool default spelled as false", "quiet", "No", true),
			ginkgo.Entry("enum default in lowercase", "level", "High", true),
			ginkgo.Entry("enum other value", "level", "Low", false),
			ginkgo.Entry("int default with a sign", "retries", "3", true),
			ginkgo.Entry("string default as written", "name", "Shoutrrr", true),
			ginkgo.Entry("string default in another case", "name", "shoutrrr", false),
			ginkgo.Entry("default that does not parse, as written", "broken", "NaN", true),
			ginkgo.Entry("default that does not parse, other value", "broken", "0", false),
			ginkgo.Entry("no default", "plain", "No", false),
			ginkgo.Entry("no default, empty value", "plain", "", true),
			ginkgo.Entry("unknown key", "unknown", "", false),
		)

		ginkgo.It("should leave values at their defaults out of the query", func() {
			config := &spelledDefaultsConfig{}
			resolver := NewPropKeyResolver(config)
			_ = resolver.SetDefaultProps(config)

			config.Broken = 0

			gomega.Expect(BuildQuery(&resolver)).To(gomega.Equal("broken=0&plain=No"))
		})
	})
	ginkgo.Describe("Setting default props", func() {
		ginkgo.When("a default tag are set for a field", func() {
			ginkgo.It("should have that value as default", func() {
				gomega.Expect(ts.Str).To(gomega.Equal("notempty"))
			})
		})
		ginkgo.When("a default tag have an invalid value", func() {
			ginkgo.It("should have that value as default", func() {
				tsb := &testStructBadDefault{}
				pkr = NewPropKeyResolver(tsb)
				err := pkr.SetDefaultProps(tsb)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})
	})
})

// GetURL returns an empty URL; the resolver tests do not use it.
func (c *titledConfig) GetURL() *url.URL { return &url.URL{} }

// SetURL ignores the URL; the resolver tests do not use it.
func (c *titledConfig) SetURL(_ *url.URL) error { return nil }

// Enums returns the enum formatter of the Level field.
func (c *spelledDefaultsConfig) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{"Level": spelledLevels}
}

// GetURL returns an empty URL; the resolver tests do not use it.
func (c *spelledDefaultsConfig) GetURL() *url.URL { return &url.URL{} }

// SetURL ignores the URL; the resolver tests do not use it.
func (c *spelledDefaultsConfig) SetURL(_ *url.URL) error { return nil }
