package main

import (
	"errors"
	"reflect"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// testStruct is used by setFieldFromString tests.
type testSetFieldStruct struct {
	Name    string
	Enabled bool
}

var _ = ginkgo.Describe("Helpers", func() {
	ginkgo.Describe("classifyType", func() {
		ginkgo.DescribeTable("returns correct type string",
			func(typ reflect.Type, isEnum bool, expected string) {
				gomega.Expect(classifyType(typ, isEnum)).To(gomega.Equal(expected))
			},
			ginkgo.Entry("string type", reflect.TypeFor[string](), false, "string"),
			ginkgo.Entry("bool type", reflect.TypeFor[bool](), false, "bool"),
			ginkgo.Entry("int type", reflect.TypeFor[int](), false, "int"),
			ginkgo.Entry("int8 type", reflect.TypeFor[int8](), false, "int"),
			ginkgo.Entry("int16 type", reflect.TypeFor[int16](), false, "int"),
			ginkgo.Entry("int32 type", reflect.TypeFor[int32](), false, "int"),
			ginkgo.Entry("int64 type", reflect.TypeFor[int64](), false, "int"),
			ginkgo.Entry("uint type", reflect.TypeFor[uint](), false, "uint"),
			ginkgo.Entry("uint8 type", reflect.TypeFor[uint8](), false, "uint"),
			ginkgo.Entry("uint16 type", reflect.TypeFor[uint16](), false, "uint"),
			ginkgo.Entry("uint32 type", reflect.TypeFor[uint32](), false, "uint"),
			ginkgo.Entry("uint64 type", reflect.TypeFor[uint64](), false, "uint"),
			ginkgo.Entry("slice type", reflect.TypeFor[[]string](), false, "array"),
			ginkgo.Entry("array type", reflect.TypeFor[[3]int](), false, "array"),
			ginkgo.Entry("map type", reflect.TypeFor[map[string]string](), false, "map"),
			ginkgo.Entry("enum override", reflect.TypeFor[int](), true, "enum"),
		)
	})

	ginkgo.Describe("urlPartToString", func() {
		ginkgo.DescribeTable("converts URL parts to string",
			func(parts []format.URLPart, expected string) {
				gomega.Expect(urlPartToString(parts)).To(gomega.Equal(expected))
			},
			ginkgo.Entry("empty parts", []format.URLPart(nil), ""),
			ginkgo.Entry("single user part", []format.URLPart{format.URLUser}, "user"),
			ginkgo.Entry("single password part", []format.URLPart{format.URLPassword}, "password"),
			ginkgo.Entry("single host part", []format.URLPart{format.URLHost}, "host"),
			ginkgo.Entry("single port part", []format.URLPart{format.URLPort}, "port"),
			ginkgo.Entry("single path part", []format.URLPart{format.URLPath}, "path"),
			ginkgo.Entry("single query part", []format.URLPart{format.URLQuery}, "query"),
			ginkgo.Entry("user and password", []format.URLPart{format.URLUser, format.URLPassword}, "user,password"),
			ginkgo.Entry("host and port", []format.URLPart{format.URLHost, format.URLPort}, "host,port"),
		)

		ginkgo.It("handles path offsets greater than URLPath", func() {
			parts := []format.URLPart{format.URLPath + 1}
			result := urlPartToString(parts)
			gomega.Expect(result).To(gomega.Equal("path1"))
		})
	})

	ginkgo.Describe("getEnumNames", func() {
		ginkgo.It("returns nil for nil formatter", func() {
			gomega.Expect(getEnumNames(nil)).To(gomega.BeNil())
		})

		ginkgo.It("returns names from formatter", func() {
			ef := format.CreateEnumFormatter([]string{"Option1", "Option2", "Option3"})
			gomega.Expect(getEnumNames(ef)).To(gomega.Equal([]string{"Option1", "Option2", "Option3"}))
		})
	})

	ginkgo.Describe("setFieldFromString", func() {
		ginkgo.It("sets string field", func() {
			s := testSetFieldStruct{}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Name", "hello")
			gomega.Expect(s.Name).To(gomega.Equal("hello"))
		})

		ginkgo.It("sets bool field from Yes", func() {
			s := testSetFieldStruct{}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Enabled", "Yes")
			gomega.Expect(s.Enabled).To(gomega.BeTrue())
		})

		ginkgo.It("sets bool field from No", func() {
			s := testSetFieldStruct{Enabled: true}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Enabled", "No")
			gomega.Expect(s.Enabled).To(gomega.BeFalse())
		})

		ginkgo.It("sets bool field from true", func() {
			s := testSetFieldStruct{}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Enabled", "true")
			gomega.Expect(s.Enabled).To(gomega.BeTrue())
		})

		ginkgo.It("sets bool field from false", func() {
			s := testSetFieldStruct{Enabled: true}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Enabled", "false")
			gomega.Expect(s.Enabled).To(gomega.BeFalse())
		})

		ginkgo.It("sets bool field from 1", func() {
			s := testSetFieldStruct{}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Enabled", "1")
			gomega.Expect(s.Enabled).To(gomega.BeTrue())
		})

		ginkgo.It("sets bool field from 0", func() {
			s := testSetFieldStruct{Enabled: true}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "Enabled", "0")
			gomega.Expect(s.Enabled).To(gomega.BeFalse())
		})

		ginkgo.It("ignores invalid field name", func() {
			s := testSetFieldStruct{}
			val := reflect.ValueOf(&s).Elem()
			setFieldFromString(val, "NonExistent", "value")
			gomega.Expect(s.Name).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("marshalError", func() {
		ginkgo.It("serializes error to JSON", func() {
			result := marshalError(errors.New("test error"))
			gomega.Expect(result).To(gomega.Equal(`{"error":"test error"}`))
		})

		ginkgo.It("handles empty error message", func() {
			result := marshalError(errors.New(""))
			gomega.Expect(result).To(gomega.Equal(`{"error":""}`))
		})
	})

	ginkgo.Describe("marshalErrorStr", func() {
		ginkgo.It("serializes string to error JSON", func() {
			result := marshalErrorStr("something failed")
			gomega.Expect(result).To(gomega.Equal(`{"error":"something failed"}`))
		})

		ginkgo.It("handles empty string", func() {
			result := marshalErrorStr("")
			gomega.Expect(result).To(gomega.Equal(`{"error":""}`))
		})

		ginkgo.It("handles string with special characters", func() {
			result := marshalErrorStr(`error with "quotes"`)
			gomega.Expect(result).To(gomega.Equal(`{"error":"error with \"quotes\""}`))
		})
	})

	ginkgo.Describe("extractScheme", func() {
		ginkgo.DescribeTable("extracts scheme from URL",
			func(rawURL, expected string) {
				gomega.Expect(extractScheme(rawURL)).To(gomega.Equal(expected))
			},
			ginkgo.Entry("discord URL", "discord://token@webhook", "discord"),
			ginkgo.Entry("teams compound scheme", "teams+https://example.com/path", "teams"),
			ginkgo.Entry("smtp URL", "smtp://user:pass@host:587", "smtp"),
			ginkgo.Entry("ntfy URL", "ntfy://ntfy.sh/topic", "ntfy"),
			ginkgo.Entry("generic URL", "generic://192.168.1.100:8123/path", "generic"),
			ginkgo.Entry("invalid URL no scheme", "invalid", ""),
			ginkgo.Entry("empty string", "", ""),
			ginkgo.Entry("only colon", "://path", ""),
		)

		ginkgo.It("handles scheme with plus separator", func() {
			gomega.Expect(extractScheme("log+http://localhost")).To(gomega.Equal("log"))
		})
	})
})
