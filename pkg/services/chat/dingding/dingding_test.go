package dingding

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var (
	service        *Service
	envDingdingURL *url.URL
)

var _ = ginkgo.BeforeSuite(func() {
	service = &Service{}
	envDingdingURL, _ = url.Parse(os.Getenv("SHOUTRRR_DINGDING_URL"))
})

var _ = ginkgo.Describe("the dingding service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("should not error out", func() {
			if envDingdingURL.String() == "" {
				return
			}

			serviceURL, _ := url.Parse(envDingdingURL.String())
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("This is an integration test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.When("given a malformed URL", func() {
		ginkgo.It("should return an error if kind is invalid", func() {
			dingdingURL := createDingdingURL(
				"badkind",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"",
				"",
				"",
				"",
			)
			expectErrorContainsMessageGivenURL("invalid service kind, must be either", dingdingURL)
		})
		ginkgo.It("should return an error if kind is worknotice and secret is missing", func() {
			dingdingURL := createDingdingURL(
				"worknotice",
				dummyAccessToken,
				"",
				"keyword",
				"",
				"",
				"",
			)
			expectErrorContainsMessageGivenURL("credentials missing from config URL", dingdingURL)
		})
		ginkgo.It("should return an error if kind is worknotice and userids is missing", func() {
			dingdingURL := createDingdingURL(
				"worknotice",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"",
				"",
				"",
				"",
			)
			expectErrorContainsMessageGivenURL("userids are required but missing from config URL", dingdingURL)
		})
	})
	ginkgo.When("given a valid service url", func() {
		ginkgo.It("should not return an error when using secret", func() {
			dingdingURL := createDingdingURL(
				"",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"",
				"",
				"",
				"",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			dingdingURL = createDingdingURL(
				"custombot",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"",
				"",
				"",
				"",
			)
			err = service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should not return an error when using keyword", func() {
			dingdingURL := createDingdingURL(
				"",
				dummyAccessToken,
				"",
				"keyword",
				"",
				"",
				"",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			dingdingURL = createDingdingURL(
				"custombot",
				dummyAccessToken,
				"",
				"keyword",
				"",
				"",
				"",
			)
			err = service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should not return an error when work notice using secret", func() {
			dingdingURL := createDingdingURL(
				"worknotice",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"",
				"keyword",
				"",
				"123456778",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
	ginkgo.When("sending a custombot message", func() {
		ginkgo.It("should override from params", func() {
			dingdingURL := createDingdingURL(
				"",
				dummyAccessToken,
				"",
				"keyword",
				"old",
				"",
				"",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			params := &types.Params{"title": "new"}

			httpmock.Activate()

			defer httpmock.DeactivateAndReset()

			apiURL := "https://oapi.dingtalk.com/robot/send"
			record := &dingtalkCustomBotPayload{}
			httpmock.RegisterResponder(
				"POST",
				apiURL,
				responderWithKeywordValidation("keyword", record, nil),
			)

			err = service.Send("test message", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(record.Markdown.Title).To(gomega.Equal("new"))
		})
		ginkgo.It("should sign when secret provided", func() {
			dingdingURL := createDingdingURL(
				"custombot",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"keyword",
				"title",
				"",
				"",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			params := &types.Params{}

			httpmock.Activate()

			defer httpmock.DeactivateAndReset()

			apiURL := "https://oapi.dingtalk.com/robot/send"
			var recordPayload dingtalkCustomBotPayload
			var recordQuery url.Values
			httpmock.RegisterResponder(
				"POST",
				apiURL,
				responderWithKeywordValidation("keyword", &recordPayload, &recordQuery),
			)

			err = service.Send("test message", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recordPayload.Markdown.Title).To(gomega.Equal("title"))
			gomega.Expect(recordQuery.Get("timestamp")).NotTo(gomega.BeEmpty())
			gomega.Expect(recordQuery.Get("sign")).NotTo(gomega.BeEmpty())
		})
		ginkgo.It("should use message if title is not provided", func() {
			dingdingURL := createDingdingURL(
				"custombot",
				dummyAccessToken,
				"",
				"keyword",
				"",
				"",
				"",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			params := &types.Params{}

			httpmock.Activate()

			defer httpmock.DeactivateAndReset()

			apiURL := "https://oapi.dingtalk.com/robot/send"
			var recordPayload dingtalkCustomBotPayload
			var recordQuery url.Values
			httpmock.RegisterResponder(
				"POST",
				apiURL,
				responderWithKeywordValidation("keyword", &recordPayload, &recordQuery),
			)

			err = service.Send("test message", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recordPayload.Markdown.Title).To(gomega.Equal("test message"))
		})
	})
	ginkgo.When("sending a worknotice message", func() {
		ginkgo.It("should be no error", func() {
			dingdingURL := createDingdingURL(
				"worknotice",
				dummyAccessToken,
				"SEC"+dummyAccessToken,
				"",
				"title",
				"",
				"123456789",
			)
			err := service.Initialize(dingdingURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			params := &types.Params{}

			httpmock.Activate()

			defer httpmock.DeactivateAndReset()

			var token string
			httpmock.RegisterResponder(
				"POST",
				"https://api.dingtalk.com/v1.0/oauth2/accessToken",
				responderWithTokenGeneration(dummyAccessToken, "SEC"+dummyAccessToken, &token),
			)
			// var recordPayload dingtalkWorkNoticePayload
			httpmock.RegisterResponder(
				"POST",
				"https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend",
				responderWithTokenValidation(&token, dummyAccessToken, nil),
			)

			err = service.Send("test message", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})

func TestDingding(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Dingding Suite")
}

// Helper function to create Dingding URLs with optional overrides.
func createDingdingURL(kind, accessToken, secret, keyword, title, template, userIDs string) *url.URL {
	query := url.Values{}
	if keyword != "" {
		query.Set("keyword", keyword)
	}
	if title != "" {
		query.Set("title", title)
	}
	if template != "" {
		query.Set("template", template)
	}
	if secret != "" {
		query.Set("secret", secret)
	}
	if kind != "" {
		query.Set("kind", kind)
	}
	if userIDs != "" {
		query.Set("userids", userIDs)
	}

	u := &url.URL{
		Scheme:   "dingding",
		Host:     accessToken,
		RawQuery: query.Encode(),
	}

	return u
}

func expectErrorContainsMessageGivenURL(msg string, dingdingURL *url.URL) {
	err := service.Initialize(dingdingURL, testutils.TestLogger())
	gomega.Expect(err).To(gomega.HaveOccurred())
	gomega.Expect(err.Error()).To(gomega.ContainSubstring(msg))
}

type dingtalkCustomBotPayload struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown"`
}

func responderWithKeywordValidation(expectedKeyword string, recordedReq *dingtalkCustomBotPayload, recordQuery *url.Values) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		if recordQuery != nil {
			*recordQuery = req.URL.Query()
		}
		// dingding always returns 200
		if req.Header.Get("Content-Type") != "application/json" {
			// refuse with param errror
			resp := httpmock.NewStringResponse(200, `{"errcode":300001,"errmsg":"param error"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		payload := &dingtalkCustomBotPayload{}
		if recordedReq != nil {
			payload = recordedReq
		}

		err := json.NewDecoder(req.Body).Decode(&payload)
		if err != nil {
			resp := httpmock.NewStringResponse(200, `{"errcode":40035,"errmsg":"缺少参数 json"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		switch payload.MsgType {
		case "text":
			if !strings.Contains(payload.Text.Content, expectedKeyword) {
				resp := httpmock.NewStringResponse(200, `{"errcode":300001,"errmsg":"错误描述:关键词不匹配;解决方案:请联系群管理员查看此机器人的关键词，并在发送的信息中包含此关键词;"}`)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			}
		case "markdown":
			if !strings.Contains(payload.Markdown.Text, expectedKeyword) {
				resp := httpmock.NewStringResponse(200, `{"errcode":300001,"errmsg":"错误描述:关键词不匹配;解决方案:请联系群管理员查看此机器人的关键词，并在发送的信息中包含此关键词;"}`)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			}
		default:
			msg := fmt.Sprintf(`{"errcode":400105,"errmsg":"错误描述: 不支持类型 msgType:%s ;解决方案:请使用支持的类型;参考链接:请参考本接口对应文档查看支持的消息类型，或者在https://open.dingtalk.com/document/ 搜索对应文档;"}`, payload.MsgType)
			resp := httpmock.NewStringResponse(200, msg)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		resp := httpmock.NewStringResponse(200, `{"errcode":0,"errmsg":"ok"}`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
}

type dingtalkWorkNoticePayload struct {
	RobotCode string   `json:"robotCode"`
	UserIDs   []string `json:"userIds"`
	MsgKey    string   `json:"msgKey"`
	MsgParam  string   `json:"msgParam"`
}

func responderWithTokenGeneration(appKey string, appSecret string, token *string) httpmock.Responder {
	return func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Content-Type") != "application/json" {
			resp := httpmock.NewStringResponse(400, `{"code":"Missingbody","requestid":"whatever","message":"body is mandatory for this action."}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		var req struct {
			AppKey    string `json:"appKey"`
			AppSecret string `json:"appSecret"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			resp := httpmock.NewStringResponse(400, `{"code":"Invalidbody","requestid":"whatever","message":"Specified parameter body is not valid. JSON parsing error"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		if appKey != req.AppKey || appSecret != req.AppSecret {
			// it returns 400, not 401 or 403
			resp := httpmock.NewStringResponse(400, `{"requestid":"whatever","code":"invalidClientIdOrSecret","message":"无效的clientId或者clientSecret"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		_token := make([]byte, 16)
		rand.Reader.Read(_token)
		*token = fmt.Sprintf("%x", _token)

		// the real api donot return code, requestid and message when successful
		resp := httpmock.NewStringResponse(200, fmt.Sprintf(`{"expireIn":7200,"accessToken":"%s"}`, *token))
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
}

func responderWithTokenValidation(
	expectedToken *string,
	expectedRobotCode string,
	recordedReq *dingtalkWorkNoticePayload,
) httpmock.Responder {
	return func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Content-Type") != "application/json" {
			resp := httpmock.NewStringResponse(400, `{"code":"Missingbody","requestid":"whatever","message":"body is mandatory for this action."}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		if r.Header.Get("x-acs-dingtalk-access-token") != *expectedToken {
			// it returns 400, not 401 or 403
			resp := httpmock.NewStringResponse(400, `{"code":"InvalidAuthentication","requestid":"whatever","message":"不合法的access_token"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		var payload *dingtalkWorkNoticePayload = recordedReq
		if payload == nil {
			payload = &dingtalkWorkNoticePayload{}
		}
		if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
			resp := httpmock.NewStringResponse(400, `{"code":"Invalidbody","requestid":"whatever","message":"Specified parameter body is not valid. JSON parsing error"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		if payload.RobotCode != expectedRobotCode {
			// real api returns "invalidParameter.robotCode.notExsit"
			resp := httpmock.NewStringResponse(400, `{"requestid":"whatever","code":"invalidParameter.robotCode.notExsit","message":"错误描述: robot 不存在；解决方案:请确认 robotCode 是否正确；"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		if len(payload.UserIDs) == 0 {
			resp := httpmock.NewStringResponse(400, `{"code":"MissinguserIds","requestid":"whatever","message":"userIds is mandatory for this action."}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		switch payload.MsgKey {
		case "sampleMarkdown":
			var msgParam markdownMessage
			if err := json.Unmarshal([]byte(payload.MsgParam), &msgParam); err != nil {
				resp := httpmock.NewStringResponse(400, `{"requestid":"whatever","code":"invalidParameter.msgParam.invalid","message":"错误描述:msgParam格式不正确;解决方案:请使用json格式;"}`)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			}
			if msgParam.Title == "" {
				resp := httpmock.NewStringResponse(400, `{"requestid":"whatever","code":"miss.param.markdownTotitle","message":"错误描述:参数 markdown --》 title 缺失，或者参数格式不正确; 解决方案:请填充上对应参数;参考链接:请参考本接口对应文档查看参数获取方式，或者在https://open.dingtalk.com/document/  搜索对应文档;"}`)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			}
		default:
			resp := httpmock.NewStringResponse(400, `{"requestid":"whatever","code":"invalidParameter.msgKey.invalid","message":"错误描述: 不支持类型 sampleMarkdoswn ;解决方案:请使用支持的类型;参考链接:请参考本接口对应文档查看支持的消息类型，或者在https://open.dingtalk.com/document/ 搜索对应文档;"}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		resp := httpmock.NewStringResponse(200, `{"flowControlledStaffIdList":[],"invalidStaffIdList":[],"processQueryKey":"whatever"}`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
}
