//go:build js && wasm

package main

import (
	"encoding/json"

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

			var errResp errorResult
			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})
	})
})
