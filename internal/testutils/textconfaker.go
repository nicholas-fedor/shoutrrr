package testutils

import (
	"bufio"
	"bytes"
	"fmt"
	"net/textproto"
	"strings"
)

type textConFaker struct {
	inputBuffer  *bytes.Buffer
	inputWriter  *bufio.Writer
	outputReader *bufio.Reader
	responses    []string
	delim        string
}

func (tcf *textConFaker) GetInput() string {
	_ = tcf.inputWriter.Flush()

	return tcf.inputBuffer.String()
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

	var convBuilder strings.Builder

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

		convBuilder.WriteString(fmt.Sprintf("  #%2d >> %50s << %-50s\n", i, query, resp))

		var convSb53 strings.Builder

		for len(resp) > 3 && resp[3] == '-' {
			responseIndex++
			resp = tcf.responses[responseIndex]
			convSb53.WriteString(fmt.Sprintf("         %50s << %-50s\n", " ", resp))
		}

		conv += convSb53.String()

		if !inSequence {
			responseIndex++
		}

		if len(resp) > 0 && resp[0] == '3' {
			inSequence = true
		}
	}

	conv += convBuilder.String()

	return conv
}

// GetClientSentences returns all the input received from the client separated by the delimiter.
func (tcf *textConFaker) GetClientSentences() []string {
	_ = tcf.inputWriter.Flush()

	return strings.Split(tcf.inputBuffer.String(), tcf.delim)
}

// CreateReadWriter returns a ReadWriter from the textConFakers internal reader and writer.
func (tcf *textConFaker) CreateReadWriter() *bufio.ReadWriter {
	return bufio.NewReadWriter(tcf.outputReader, tcf.inputWriter)
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
		responses: responses,
		delim:     delim,
	}
	tcfaker.init()

	// rx := iotest.NewReadLogger("TextConRx", tcfaker.outputReader)
	// tx := iotest.NewWriteLogger("TextConTx", tcfaker.inputWriter)
	// faker := CreateIOFaker(rx, tx)
	faker := ioFaker{
		ReadWriter: tcfaker.CreateReadWriter(),
	}

	return textproto.NewConn(faker), &tcfaker
}
