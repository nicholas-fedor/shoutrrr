package dingding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

type textMessage struct {
	Content string `json:"content"`
}

type markdownMessage struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

const defaultCustomBotTemplate = `{
	"msgtype": "markdown",
	"markdown": {{ toMarkdownMsgParam .Title .Message }}
}`

const defaultWorkNoticeTemplate = `{
	"robotCode": "{{ .AccessToken }}",
	"userIds": {{ toJSON .UserIDs }},
	"msgKey": "sampleMarkdown",
	"msgParam": {{ toMarkdownMsgParam .Title .Message | toJSON }}
}`

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func toMarkdownMsgParam(title, message string) (string, error) {
	msg := markdownMessage{
		Title: title,
		Text:  message,
	}

	return toJSON(msg)
}

var tmplfuncMap = template.FuncMap{
	"toMarkdownMsgParam": toMarkdownMsgParam,
	"toJSON":             toJSON,
}

func makeTemplate(kind string, tmplStr string) (*template.Template, error) {
	var err error
	// tmpl := &standard.Templater{}
	if tmplStr == "" {
		switch kind {
		case "custombot":
			tmplStr = defaultCustomBotTemplate
		case "worknotice":
			tmplStr = defaultWorkNoticeTemplate
		default:
			return nil, ErrInvalidKind
		}
	}

	tmpl, err := template.New("payload").Funcs(tmplfuncMap).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	return tmpl, err
}

func (c Config) createPayload(title, message string) ([]byte, error) {
	// prepare message
	if title == "" {
		// title is mandatory for markdown, so we fix it here
		title = message
	}
	if c.Keyword != "" && !strings.Contains(message, c.Keyword) {
		message = message + "\r\n<!--" + c.Keyword + "-->"
	}

	buf := bytes.Buffer{}

	err := c.tmpl.Execute(&buf, struct {
		AccessToken string
		UserIDs     []string
		Title       string
		Message     string
	}{
		AccessToken: c.AccessToken,
		UserIDs:     strings.Split(c.UserIDs, ","),
		Title:       title,
		Message:     message,
	})

	return buf.Bytes(), err
}
