package matrix

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix/mocks"
)

// testLogger is a simple logger for testing purposes.
type testLogger struct {
	formatted string
}

// errorReadCloser is a ReadCloser that always returns an error on read.
type errorReadCloser struct{}

var _ = ginkgo.Describe("client", func() {
	ginkgo.Describe("newClient", func() {
		ginkgo.It("should create a client with https scheme by default", func() {
			c := newClient("matrix.example.com", false, nil)
			gomega.Expect(c).ToNot(gomega.BeNil())
			gomega.Expect(c.apiURL.Scheme).To(gomega.Equal("https"))
			gomega.Expect(c.apiURL.Host).To(gomega.Equal("matrix.example.com"))
		})

		ginkgo.It("should create a client with http scheme when TLS is disabled", func() {
			c := newClient("matrix.example.com", true, nil)
			gomega.Expect(c).ToNot(gomega.BeNil())
			gomega.Expect(c.apiURL.Scheme).To(gomega.Equal("http"))
		})

		ginkgo.It("should use discard logger when nil logger is provided", func() {
			c := newClient("matrix.example.com", false, nil)
			gomega.Expect(c.logger).ToNot(gomega.BeNil())
		})
	})

	ginkgo.Describe("apiGet", func() {
		ginkgo.It("should handle HTTP errors from the client", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiGet("/_matrix/client/v3/login", resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle HTTP error status codes", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid request","errcode":"M_INVALID_PARAM"}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiGet("/_matrix/client/v3/login", resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid request"))
		})

		ginkgo.It("should handle HTTP error status without JSON response", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte("internal server error"))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiGet("/_matrix/client/v3/login", resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("HTTP"))
		})

		ginkgo.It("should handle JSON unmarshal error on response", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("not valid json"))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiGet("/_matrix/client/v3/login", resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle body read error", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       &errorReadCloser{},
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiGet("/_matrix/client/v3/login", resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("apiPost", func() {
		ginkgo.It("should handle HTTP errors from the client", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiPost("/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle HTTP error status codes", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid request","errcode":"M_INVALID_PARAM"}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiPost("/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle JSON unmarshal error on response", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("not valid json"))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiPost("/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("getJoinedRooms", func() {
		ginkgo.It("should return empty slice on API error", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			rooms, err := c.getJoinedRooms()
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(rooms).To(gomega.BeEmpty())
		})

		ginkgo.It("should return rooms on success", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"joined_rooms":["!room1:matrix.example.com","!room2:matrix.example.com"]}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			rooms, err := c.getJoinedRooms()
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(rooms).To(gomega.HaveLen(2))
		})
	})

	ginkgo.Describe("joinRoom", func() {
		ginkgo.It("should return error on API failure", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			roomID, err := c.joinRoom("#test:matrix.example.com")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(roomID).To(gomega.BeEmpty())
		})

		ginkgo.It("should return room ID on success", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"room_id":"!joinedroom:matrix.example.com"}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			roomID, err := c.joinRoom("#test:matrix.example.com")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(roomID).To(gomega.Equal("!joinedroom:matrix.example.com"))
		})
	})

	ginkgo.Describe("logf", func() {
		ginkgo.It("should call logger Printf", func() {
			mockLogger := &testLogger{}
			c := &client{
				logger: mockLogger,
			}

			c.logf("test %s", "message")
			gomega.Expect(mockLogger.formatted).To(gomega.ContainSubstring("test"))
		})
	})

	ginkgo.Describe("login", func() {
		ginkgo.It("should return error when API call fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.login("user", "password")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should return error when no supported login flows", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"flows":[{"type":"m.login.unknown"}]}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.login("user", "password")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("login flows"))
		})

		ginkgo.It("should use token login when user is empty", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"flows":[{"type":"m.login.token"}]}`))),
			}, nil).Once()
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"access_token":"testtoken123","home_server":"matrix.example.com","user_id":"@user:matrix.example.com"}`))),
			}, nil).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.login("", "testtoken123")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(c.accessToken).To(gomega.Equal("testtoken123"))
		})
	})

	ginkgo.Describe("loginPassword", func() {
		ginkgo.It("should return error when API call fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.loginPassword("user", "password")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should successfully login with password", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"access_token":"mytoken123","home_server":"matrix.example.com","user_id":"@user:matrix.example.com","device_id":"TESTDEVICE"}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.loginPassword("user", "password")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(c.accessToken).To(gomega.Equal("mytoken123"))
		})
	})

	ginkgo.Describe("loginToken", func() {
		ginkgo.It("should return error when API call fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.loginToken("token")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should successfully login with token", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"access_token":"token123","home_server":"matrix.example.com","user_id":"@user:matrix.example.com"}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.loginToken("token123")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(c.accessToken).To(gomega.Equal("token123"))
		})
	})

	ginkgo.Describe("sendMessage", func() {
		ginkgo.It("should call sendToJoinedRooms when no rooms specified", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendMessage("test message", nil)
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})

		ginkgo.It("should call sendToExplicitRooms when rooms are specified", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendMessage("test message", []string{"#room:matrix.example.com"})
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})
	})

	ginkgo.Describe("sendMessageToRoom", func() {
		ginkgo.It("should return error when API call fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			err := c.sendMessageToRoom("test message", "!room:matrix.example.com")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("sendToExplicitRooms", func() {
		ginkgo.It("should return errors when joining rooms fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToExplicitRooms([]string{"#room:matrix.example.com"}, "test message")
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})

		ginkgo.It("should send message successfully when joining succeeds", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// First call: joinRoom
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"room_id":"!roomid:matrix.example.com"}`))),
			}, nil).Once()
			// Second call: sendMessageToRoom
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"event_id":"$eventid"}`))),
			}, nil).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToExplicitRooms([]string{"#room:matrix.example.com"}, "test message")
			gomega.Expect(errs).To(gomega.BeEmpty())
		})

		ginkgo.It("should return error when sending message fails after joining", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// First call: joinRoom - success
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"room_id":"!roomid:matrix.example.com"}`))),
			}, nil).Once()
			// Second call: sendMessageToRoom - failure
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("send error")).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToExplicitRooms([]string{"#room:matrix.example.com"}, "test message")
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})
	})

	ginkgo.Describe("sendToJoinedRooms", func() {
		ginkgo.It("should return error when getJoinedRooms fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("mock error"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToJoinedRooms("test message")
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})

		ginkgo.It("should send message to all joined rooms", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// First call: getJoinedRooms
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"joined_rooms":["!room1:matrix.example.com"]}`))),
			}, nil).Once()
			// Second call: sendMessageToRoom
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"event_id":"$eventid"}`))),
			}, nil).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToJoinedRooms("test message")
			gomega.Expect(errs).To(gomega.BeEmpty())
		})

		ginkgo.It("should return errors when sending to joined rooms fails", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// First call: getJoinedRooms - success
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"joined_rooms":["!room1:matrix.example.com"]}`))),
			}, nil).Once()
			// Second call: sendMessageToRoom - failure
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("send error")).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToJoinedRooms("test message")
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})
	})

	ginkgo.Describe("updateAccessToken", func() {
		ginkgo.It("should update the access token in query params", func() {
			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				accessToken: "test-token",
				logger:      &testLogger{},
			}

			c.updateAccessToken()
			gomega.Expect(c.apiURL.Query().Get("access_token")).To(gomega.Equal("test-token"))
		})
	})

	ginkgo.Describe("useToken", func() {
		ginkgo.It("should set the access token", func() {
			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				logger: &testLogger{},
			}

			c.useToken("my-token")
			gomega.Expect(c.accessToken).To(gomega.Equal("my-token"))
			gomega.Expect(c.apiURL.Query().Get("access_token")).To(gomega.Equal("my-token"))
		})
	})
})

func (m *testLogger) Print(args ...any) {
	// no-op
}

func (m *testLogger) Printf(format string, args ...any) {
	m.formatted = format
}

func (m *testLogger) Println(args ...any) {
	// no-op
}

func (e *errorReadCloser) Close() error {
	return nil
}

func (e *errorReadCloser) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}
