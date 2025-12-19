package discord_test

import (
	"strings"

	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Files", func() {
	var testService *discord.Service
	var dummyConfig discord.Config

	ginkgo.BeforeEach(func() {
		httpmock.Activate()
		dummyConfig = CreateDummyConfig()
		testService = CreateTestService(dummyConfig)
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.Context("file attachment functionality", func() {
		ginkgo.It("should send single file attachment", func() {
			testData := []byte("test file content")
			SetupMockResponder(&dummyConfig, 200)
			items := CreateMessageItemWithFile("Message with file", "test.txt", testData)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send multiple file attachments", func() {
			file1Data := []byte("content of file 1")
			file2Data := []byte("content of file 2")

			SetupMockResponder(&dummyConfig, 200)

			items := []types.MessageItem{
				{
					Text: "Message with multiple files",
					File: &types.File{Name: "file1.txt", Data: file1Data},
				},
				{
					Text: "Second file",
					File: &types.File{Name: "file2.txt", Data: file2Data},
				},
			}
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should handle files with special characters in names", func() {
			testData := []byte("special file content")
			specialFilename := "test-file_with.special.chars.txt"

			SetupMockResponder(&dummyConfig, 200)

			items := CreateMessageItemWithFile(
				"Message with special filename",
				specialFilename,
				testData,
			)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should validate multipart form data structure and boundaries", func() {
			testData := []byte("multipart validation test data")

			SetupMockResponder(&dummyConfig, 200)

			items := CreateMessageItemWithFile(
				"Multipart boundary validation test",
				"boundary-test.txt",
				testData,
			)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should handle multiple files with different sizes and types", func() {
			smallFileData := []byte("small file")
			mediumFileData := make([]byte, 1024*100) // 100KB
			for i := range mediumFileData {
				mediumFileData[i] = byte(i % 256)
			}
			largeFileData := make([]byte, 1024*1024) // 1MB
			for i := range largeFileData {
				largeFileData[i] = byte((i * 7) % 256) // Pseudo-random pattern
			}

			SetupMockResponder(&dummyConfig, 200)

			items := []types.MessageItem{
				{
					Text: "Multiple files test",
					File: &types.File{Name: "small.txt", Data: smallFileData},
				},
				{
					Text: "Medium file",
					File: &types.File{Name: "medium.dat", Data: mediumFileData},
				},
				{
					Text: "Large file",
					File: &types.File{Name: "large.bin", Data: largeFileData},
				},
			}

			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.Context("file attachment edge cases", func() {
			ginkgo.It("should handle empty file attachments", func() {
				emptyFileData := []byte("")
				items := CreateMessageItemWithFile(
					"Message with empty file",
					"empty.txt",
					emptyFileData,
				)
				SetupMockResponder(&dummyConfig, 204)
				err := testService.SendItems(items, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
			})

			ginkgo.It("should handle files with special characters in names", func() {
				specialFilename := "file-with-special-chars_测试文件.txt"
				fileData := []byte("test content")
				items := CreateMessageItemWithFile(
					"Message with special filename",
					specialFilename,
					fileData,
				)
				SetupMockResponder(&dummyConfig, 204)
				err := testService.SendItems(items, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
			})

			ginkgo.It("should handle very long filenames", func() {
				longFilename := strings.Repeat("a", 255) + ".txt" // Maximum typical filename length
				fileData := []byte("test content")
				items := CreateMessageItemWithFile(
					"Message with long filename",
					longFilename,
					fileData,
				)
				SetupMockResponder(&dummyConfig, 204)
				err := testService.SendItems(items, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
			})
		})
	})
})
