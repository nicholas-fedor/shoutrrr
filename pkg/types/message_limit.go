package types

// MessageLimit is used for declaring the payload limits for services upstream APIs.
type MessageLimit struct {
	// ChunkSize is the maximum size of a single chunk.
	ChunkSize int
	// TotalChunkSize is the maximum combined size of the chunks in one
	// message.
	TotalChunkSize int

	// Maximum number of chunks (including the last chunk for meta data)
	ChunkCount int
}
