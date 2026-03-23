//go:build js && wasm

package main

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Send", func() {
	ginkgo.Describe("sendString", func() {
		ginkgo.It("returns success for valid logger URL", func() {
			result := sendString("logger://", "test message")

			gomega.Expect(result).To(gomega.Equal(`{"success":true}`))
		})

		ginkgo.It("returns error for invalid URL", func() {
			result := sendString("invalid://url", "test message")

			gomega.Expect(result).To(gomega.ContainSubstring("error"))
		})
	})
})
