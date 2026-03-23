//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Main JS Bindings", func() {
	ginkgo.Describe("getServices", func() {
		ginkgo.It("returns JSON array of services", func() {
			result := getServices(js.Value{}, nil)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("discord"))
		})
	})

	ginkgo.Describe("getConfigSchema", func() {
		ginkgo.It("returns error when no args provided", func() {
			result := getConfigSchema(js.Value{}, nil)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("error"))
		})
	})

	ginkgo.Describe("parseURL", func() {
		ginkgo.It("returns error when no args provided", func() {
			result := parseURL(js.Value{}, nil)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("error"))
		})
	})

	ginkgo.Describe("generateURL", func() {
		ginkgo.It("returns error when insufficient args", func() {
			result := generateURL(js.Value{}, nil)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("error"))
		})
	})

	ginkgo.Describe("validateURL", func() {
		ginkgo.It("returns error when no args provided", func() {
			result := validateURL(js.Value{}, nil)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("error"))
		})
	})

	ginkgo.Describe("send", func() {
		ginkgo.It("returns rejected promise when insufficient args", func() {
			result := send(js.Value{}, nil)
			_, ok := result.(js.Value)
			gomega.Expect(ok).To(gomega.BeTrue())
		})
	})
})
