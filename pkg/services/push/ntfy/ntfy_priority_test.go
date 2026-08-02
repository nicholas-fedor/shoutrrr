package ntfy

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Priority", func() {
	ginkgo.Describe("String", func() {
		ginkgo.It("should return 'Min' for PriorityMin", func() {
			gomega.Expect(PriorityMin.String()).To(gomega.Equal("Min"))
		})

		ginkgo.It("should return 'Low' for PriorityLow", func() {
			gomega.Expect(PriorityLow.String()).To(gomega.Equal("Low"))
		})

		ginkgo.It("should return 'Default' for PriorityDefault", func() {
			gomega.Expect(PriorityDefault.String()).To(gomega.Equal("Default"))
		})

		ginkgo.It("should return 'High' for PriorityHigh", func() {
			gomega.Expect(PriorityHigh.String()).To(gomega.Equal("High"))
		})

		ginkgo.It("should return 'Max' for PriorityMax", func() {
			gomega.Expect(PriorityMax.String()).To(gomega.Equal("Max"))
		})

		ginkgo.It("should return formatted string for unknown values", func() {
			unknown := priority(99)
			result := unknown.String()
			gomega.Expect(result).NotTo(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("Priority values", func() {
		ginkgo.It("should have correct numeric values", func() {
			gomega.Expect(int(PriorityMin)).To(gomega.Equal(1))
			gomega.Expect(int(PriorityLow)).To(gomega.Equal(2))
			gomega.Expect(int(PriorityDefault)).To(gomega.Equal(3))
			gomega.Expect(int(PriorityHigh)).To(gomega.Equal(4))
			gomega.Expect(int(PriorityMax)).To(gomega.Equal(5))
		})
	})

	ginkgo.Describe("Priority enum formatter", func() {
		ginkgo.It("should have non-nil Enum formatter", func() {
			gomega.Expect(Priority.Enum).NotTo(gomega.BeNil())
		})

		ginkgo.It("should map string names to values", func() {
			gomega.Expect(Priority.Enum.Parse("High")).To(gomega.Equal(int(PriorityHigh)))
		})
	})
})
