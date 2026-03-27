//go:build js && wasm

package main

import (
	"syscall/js"
	"time"

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

		ginkgo.It("returns schema for discord", func() {
			args := []js.Value{js.ValueOf("discord")}
			result := getConfigSchema(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("discord"))
			gomega.Expect(str).To(gomega.ContainSubstring("WebhookID"))
		})

		ginkgo.It("returns error for invalid service", func() {
			args := []js.Value{js.ValueOf("nonexistent")}
			result := getConfigSchema(js.Value{}, args)
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

		ginkgo.It("parses discord URL", func() {
			args := []js.Value{js.ValueOf("discord://token@123456789")}
			result := parseURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("discord"))
			gomega.Expect(str).To(gomega.ContainSubstring("config"))
		})

		ginkgo.It("parses ntfy URL", func() {
			args := []js.Value{js.ValueOf("ntfy://ntfy.sh/mytopic")}
			result := parseURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("ntfy"))
		})

		ginkgo.It("returns error for invalid URL", func() {
			args := []js.Value{js.ValueOf("not-a-valid-url")}
			result := parseURL(js.Value{}, args)
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

		ginkgo.It("generates discord URL", func() {
			args := []js.Value{
				js.ValueOf("discord"),
				js.ValueOf(`{"WebhookID":"123456789","Token":"mytoken"}`),
			}
			result := generateURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("url"))
			gomega.Expect(str).To(gomega.ContainSubstring("discord://"))
		})

		ginkgo.It("returns error for invalid service", func() {
			args := []js.Value{
				js.ValueOf("nonexistent"),
				js.ValueOf("{}"),
			}
			result := generateURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("error"))
		})

		ginkgo.It("returns error for invalid JSON", func() {
			args := []js.Value{
				js.ValueOf("discord"),
				js.ValueOf("not-json"),
			}
			result := generateURL(js.Value{}, args)
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

		ginkgo.It("validates discord URL", func() {
			args := []js.Value{js.ValueOf("discord://token@123456789")}
			result := validateURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("valid"))
		})

		ginkgo.It("validates ntfy URL", func() {
			args := []js.Value{js.ValueOf("ntfy://ntfy.sh/mytopic")}
			result := validateURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("valid"))
		})

		ginkgo.It("returns error for invalid URL", func() {
			args := []js.Value{js.ValueOf("not-valid")}
			result := validateURL(js.Value{}, args)
			str, ok := result.(string)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(str).To(gomega.ContainSubstring("error"))
		})
	})

	ginkgo.Describe("send", func() {
		ginkgo.It("returns rejected Promise for insufficient args", func() {
			result := send(js.Value{}, nil)
			promise, ok := result.(js.Value)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(promise.Get("constructor").Get("name").String()).To(gomega.Equal("Promise"))

			// Synchronous rejection: verify rejection contains expected error.
			rejected := false

			promise.Call("catch", js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
				rejected = true
				gomega.Expect(args[0].String()).To(gomega.ContainSubstring("missing arguments"))

				return nil
			}))

			gomega.Expect(rejected).To(gomega.BeTrue())
		})

		ginkgo.It("returns rejected Promise for invalid URL", func() {
			args := []js.Value{js.ValueOf("not-a-valid-url"), js.ValueOf("test message")}
			result := send(js.Value{}, args)
			promise, ok := result.(js.Value)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(promise.Get("constructor").Get("name").String()).To(gomega.Equal("Promise"))

			// Async rejection: wait for goroutine via channel.
			done := make(chan string, 1)

			promise.Call("catch", js.FuncOf(func(_ js.Value, catchArgs []js.Value) interface{} {
				done <- catchArgs[0].String()

				return nil
			}))

			select {
			case errMsg := <-done:
				gomega.Expect(errMsg).To(gomega.ContainSubstring("error"))
			case <-time.After(5 * time.Second):
				ginkgo.Fail("timed out waiting for Promise rejection")
			}
		})

		ginkgo.It("returns rejected Promise for empty message", func() {
			args := []js.Value{js.ValueOf("logger://"), js.ValueOf("")}
			result := send(js.Value{}, args)
			promise, ok := result.(js.Value)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(promise.Get("constructor").Get("name").String()).To(gomega.Equal("Promise"))

			done := make(chan string, 1)

			promise.Call("catch", js.FuncOf(func(_ js.Value, catchArgs []js.Value) interface{} {
				done <- catchArgs[0].String()

				return nil
			}))

			select {
			case errMsg := <-done:
				gomega.Expect(errMsg).To(gomega.ContainSubstring("error"))
			case <-time.After(5 * time.Second):
				ginkgo.Fail("timed out waiting for Promise rejection")
			}
		})
	})
})
