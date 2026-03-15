package testutils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strings"
)

// Eavesdropper is an interface that provides a way to get a summarized output of a connection RX and TX.
type Eavesdropper interface {
	GetConversation(includeGreeting bool) string
	GetClientSentences() []string
}

type ioFaker struct {
	io.ReadWriter
}

type failWriter struct {
	writeLimit int
	writeCount int
}

type textConFaker struct {
	inputBuffer  *bytes.Buffer
	inputWriter  *bufio.Writer
	outputReader *bufio.Reader
	responses    []string
	delim        string
}

// ErrWriteLimitReached is returned when the write limit has been reached.
var ErrWriteLimitReached = errors.New("reached write limit")

// Close is just a dummy function to implement the io.Closer interface.
func (iof ioFaker) Close() error {
	return nil
}

// Close is just a dummy function to implement io.Closer.
func (fw *failWriter) Close() error {
	return nil
}

// Write returns an error if the write limit has been reached.
func (fw *failWriter) Write(data []byte) (int, error) {
	fw.writeCount++
	if fw.writeCount > fw.writeLimit {
		return 0, fmt.Errorf("%w: %d", ErrWriteLimitReached, fw.writeLimit)
	}

	return len(data), nil
}

// CreateFailWriter returns a io.WriteCloser that returns an error after the amount of writes indicated by writeLimit.
func CreateFailWriter(writeLimit int) io.WriteCloser {
	return &failWriter{
		writeLimit: writeLimit,
		writeCount: 0,
	}
}

// CreateReadWriter returns a ReadWriter from the textConFakers internal reader and writer.
func (tcf *textConFaker) CreateReadWriter() *bufio.ReadWriter {
	return bufio.NewReadWriter(tcf.outputReader, tcf.inputWriter)
}

// GetClientSentences returns all the input received from the client separated by the delimiter.
func (tcf *textConFaker) GetClientSentences() []string {
	_ = tcf.inputWriter.Flush()

	return strings.Split(tcf.inputBuffer.String(), tcf.delim)
}

// GetConversation returns the input and output streams as a conversation.
func (tcf *textConFaker) GetConversation(includeGreeting bool) string {
	conv := ""
	inSequence := false
	input := strings.Split(tcf.GetInput(), tcf.delim)
	responseIndex := 0

	if includeGreeting {
		conv += fmt.Sprintf("    %-55s << %-50s\n", "(server greeting)", tcf.responses[0])
		responseIndex = 1
	}

	// conversationBuilder accumulates the conversation output in correct order: main lines followed by their continuations.
	var conversationBuilder strings.Builder

	for i, query := range input {
		if query == "." {
			inSequence = false
		}

		resp := ""
		if len(tcf.responses) > responseIndex && !inSequence {
			resp = tcf.responses[responseIndex]
		}

		if query == "" && resp == "" && i == len(input)-1 {
			break
		}

		fmt.Fprintf(&conversationBuilder, "  #%2d >> %50s << %-50s\n", i, query, resp)

		for len(resp) > 3 && resp[3] == '-' {
			responseIndex++
			if responseIndex >= len(tcf.responses) {
				break
			}

			resp = tcf.responses[responseIndex]
			fmt.Fprintf(&conversationBuilder, "         %50s << %-50s\n", " ", resp)
		}

		if !inSequence {
			responseIndex++
		}

		if resp != "" && resp[0] == '3' {
			inSequence = true
		}
	}

	conv += conversationBuilder.String()

	return conv
}

func (tcf *textConFaker) GetInput() string {
	_ = tcf.inputWriter.Flush()

	return tcf.inputBuffer.String()
}

func (tcf *textConFaker) init() {
	tcf.inputBuffer = &bytes.Buffer{}
	stringReader := strings.NewReader(strings.Join(tcf.responses, tcf.delim))
	tcf.outputReader = bufio.NewReader(stringReader)
	tcf.inputWriter = bufio.NewWriter(tcf.inputBuffer)
}

// CreateTextConFaker returns a textproto.Conn to fake textproto based connections.
func CreateTextConFaker(responses []string, delim string) (*textproto.Conn, Eavesdropper) {
	tcfaker := textConFaker{
		inputBuffer:  nil,
		inputWriter:  nil,
		outputReader: nil,
		responses:    responses,
		delim:        delim,
	}
	tcfaker.init()

	faker := ioFaker{
		ReadWriter: tcfaker.CreateReadWriter(),
	}

	return textproto.NewConn(faker), &tcfaker
}
