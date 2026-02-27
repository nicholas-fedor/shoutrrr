package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Partition Message", func() {
	limits := types.MessageLimit{
		ChunkSize:      2000,
		TotalChunkSize: 6000,
		ChunkCount:     10,
	}

	ginkgo.When("given a message that exceeds the max length", func() {
		ginkgo.When("not splitting by lines", func() {
			ginkgo.It("should return a payload with chunked messages", func() {
				items, _ := testPartitionMessage(42)
				gomega.Expect(items[0].Text).To(gomega.HaveLen(1994))
				gomega.Expect(items[1].Text).To(gomega.HaveLen(1999))
				gomega.Expect(items[2].Text).To(gomega.HaveLen(205))
			})
			ginkgo.It("omit characters above total max", func() {
				items, _ := testPartitionMessage(62)
				gomega.Expect(items[0].Text).To(gomega.HaveLen(1994))
				gomega.Expect(items[1].Text).To(gomega.HaveLen(1999))
				gomega.Expect(items[2].Text).To(gomega.HaveLen(1999))
				gomega.Expect(items[3].Text).To(gomega.HaveLen(5))
			})
			ginkgo.It("should handle messages with a size modulus of chunksize", func() {
				items, _ := testPartitionMessage(20)
				// Last word fits in the chunk size
				gomega.Expect(items[0].Text).To(gomega.HaveLen(2000))

				items, _ = testPartitionMessage(40)
				// Now the last word of the first chunk will be concatenated with
				// the first word of the second chunk, and so it does not fit in the chunk anymore
				gomega.Expect(items[0].Text).To(gomega.HaveLen(1994))
				gomega.Expect(items[1].Text).To(gomega.HaveLen(1999))
				gomega.Expect(items[2].Text).To(gomega.HaveLen(5))
			})
			ginkgo.When("the message is empty", func() {
				ginkgo.It("should return no items", func() {
					items, _ := testPartitionMessage(0)
					gomega.Expect(items).To(gomega.BeEmpty())
				})
			})
			ginkgo.When("given an input without whitespace", func() {
				ginkgo.It("should not crash, regardless of length", func() {
					unalignedLimits := types.MessageLimit{
						ChunkSize:      1997,
						ChunkCount:     11,
						TotalChunkSize: 5631,
					}

					testString := ""
					for inputLen := 1; inputLen < 8000; inputLen++ {
						// add a rune to the string using a repeatable pattern (single digit hex of position)
						testString += strconv.FormatInt(int64(inputLen%16), 16)
						items, omitted := PartitionMessage(testString, unalignedLimits, 7)
						included := 0

						for ii, item := range items {
							expectedSize := unalignedLimits.ChunkSize

							// The last chunk might be smaller than the preceding chunks
							if ii == len(items)-1 {
								// the chunk size is the remainder of, the total size,
								// or the max size, whatever is smallest,
								// and the previous chunk sizes
								chunkSize := Min(
									inputLen,
									unalignedLimits.TotalChunkSize,
								) % unalignedLimits.ChunkSize
								// if the "rest" of the runes needs another chunk
								if chunkSize > 0 {
									// expect the chunk to contain the "rest" of the runes
									expectedSize = chunkSize
								}
								// the last chunk should never be empty, so treat it as one of the full ones
							}

							// verify the data, but only on the last chunk to reduce test time
							if ii == len(items)-1 {
								for ri, r := range item.Text {
									runeOffset := (len(item.Text) - ri) - 1
									runeVal, err := strconv.ParseInt(string(r), 16, 64)
									expectedLen := Min(inputLen, unalignedLimits.TotalChunkSize)
									expectedVal := (expectedLen - runeOffset) % 16

									gomega.Expect(err).ToNot(gomega.HaveOccurred())
									gomega.Expect(runeVal).To(gomega.Equal(int64(expectedVal)))
								}
							}

							included += len(item.Text)
							gomega.Expect(item.Text).To(gomega.HaveLen(expectedSize))
						}

						gomega.Expect(omitted + included).To(gomega.Equal(inputLen))
					}
				})
			})
		})
		ginkgo.When("splitting by lines", func() {
			ginkgo.It("should return a payload with chunked messages", func() {
				batches := testMessageItemsFromLines(18, limits, 2)
				items := batches[0]

				gomega.Expect(items[0].Text).To(gomega.HaveLen(200))
				gomega.Expect(items[8].Text).To(gomega.HaveLen(200))
			})
			ginkgo.When("the message items exceed the limits", func() {
				ginkgo.It("should split items into multiple batches", func() {
					batches := testMessageItemsFromLines(21, limits, 2)

					for b, chunks := range batches {
						_, _ = fmt.Fprintf(
							ginkgo.GinkgoWriter,
							"Batch #%v: (%v chunks)\n",
							b,
							len(chunks),
						)
						for c, chunk := range chunks {
							_, _ = fmt.Fprintf(
								ginkgo.GinkgoWriter,
								" - Chunk #%v: (%v runes)\n",
								c,
								len(chunk.Text),
							)
						}
					}

					gomega.Expect(batches).To(gomega.HaveLen(2))
				})
			})
			ginkgo.It("should trim characters above chunk size", func() {
				hundreds := 42
				repeat := 21
				batches := testMessageItemsFromLines(hundreds, limits, repeat)
				items := batches[0]

				gomega.Expect(items[0].Text).To(gomega.HaveLen(limits.ChunkSize))
				gomega.Expect(items[1].Text).To(gomega.HaveLen(limits.ChunkSize))
			})
		})
	})
})

const hundredChars = "this string is exactly (to the letter) a hundred characters long which will make the send func error"

// testMessageItemsFromLines generates message item batches from repeated text with line breaks.
func testMessageItemsFromLines(
	hundreds int,
	limits types.MessageLimit,
	repeat int,
) [][]types.MessageItem {
	builder := strings.Builder{}
	ri := 0

	for range hundreds {
		builder.WriteString(hundredChars)

		ri++
		if ri == repeat {
			builder.WriteRune('\n')

			ri = 0
		}
	}

	return MessageItemsFromLines(builder.String(), limits)
}

// testPartitionMessage partitions repeated text into message items.
func testPartitionMessage(hundreds int) ([]types.MessageItem, int) {
	limits := types.MessageLimit{
		ChunkSize:      2000,
		TotalChunkSize: 6000,
		ChunkCount:     10,
	}
	builder := strings.Builder{}

	for range hundreds {
		builder.WriteString(hundredChars)
	}

	items, omitted := PartitionMessage(builder.String(), limits, 100)
	contentSize := Min(hundreds*100, limits.TotalChunkSize)
	expectedOmitted := Max(0, (hundreds*100)-contentSize)

	gomega.ExpectWithOffset(0, omitted).To(gomega.Equal(expectedOmitted))

	return items, omitted
}
