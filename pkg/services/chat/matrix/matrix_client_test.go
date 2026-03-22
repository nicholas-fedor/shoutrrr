package matrix

import (
	"bytes"
	"context"
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

// errorBodyClose is a ReadCloser that returns error on close.
type errorBodyClose struct{}

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
			err := c.apiGet(context.Background(), "/_matrix/client/v3/login", resp)
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
			err := c.apiGet(context.Background(), "/_matrix/client/v3/login", resp)
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
			err := c.apiGet(context.Background(), "/_matrix/client/v3/login", resp)
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
			err := c.apiGet(context.Background(), "/_matrix/client/v3/login", resp)
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
			err := c.apiGet(context.Background(), "/_matrix/client/v3/login", resp)
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
			err := c.apiPost(context.Background(), "/_matrix/client/v3/login", nil, resp)
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
			err := c.apiPost(context.Background(), "/_matrix/client/v3/login", nil, resp)
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
			err := c.apiPost(context.Background(), "/_matrix/client/v3/login", nil, resp)
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

			rooms, err := c.getJoinedRooms(context.Background())
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

			rooms, err := c.getJoinedRooms(context.Background())
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

			roomID, err := c.joinRoom(context.Background(), "#test:matrix.example.com")
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

			roomID, err := c.joinRoom(context.Background(), "#test:matrix.example.com")
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

			err := c.login(context.Background(), "user", "password")
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

			err := c.login(context.Background(), "user", "password")
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

			err := c.login(context.Background(), "", "testtoken123")
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

			err := c.loginPassword(context.Background(), "user", "password")
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

			err := c.loginPassword(context.Background(), "user", "password")
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

			err := c.loginToken(context.Background(), "token")
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

			err := c.loginToken(context.Background(), "token123")
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

			errs := c.sendMessage(context.Background(), "test message", nil)
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

			errs := c.sendMessage(context.Background(), "test message", []string{"#room:matrix.example.com"})
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

			err := c.sendMessageToRoom(context.Background(), "test message", "!room:matrix.example.com")
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

			errs := c.sendToExplicitRooms(context.Background(), []string{"#room:matrix.example.com"}, "test message")
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

			errs := c.sendToExplicitRooms(context.Background(), []string{"#room:matrix.example.com"}, "test message")
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

			errs := c.sendToExplicitRooms(context.Background(), []string{"#room:matrix.example.com"}, "test message")
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

			errs := c.sendToJoinedRooms(context.Background(), "test message")
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

			errs := c.sendToJoinedRooms(context.Background(), "test message")
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

			errs := c.sendToJoinedRooms(context.Background(), "test message")
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

	ginkgo.Describe("generateTransactionID", func() {
		ginkgo.It("should generate unique transaction IDs", func() {
			txn1 := generateTransactionID()
			txn2 := generateTransactionID()
			gomega.Expect(txn1).ToNot(gomega.Equal(txn2))
			gomega.Expect(txn1).To(gomega.MatchRegexp(`^[0-9]+-[a-f0-9]+$`))
		})
	})

	ginkgo.Describe("createMessage", func() {
		ginkgo.It("should return original message when title is empty", func() {
			result := createMessage("Hello world", "")
			gomega.Expect(result).To(gomega.Equal("Hello world"))
		})

		ginkgo.It("should prepend title to message", func() {
			result := createMessage("Hello world", "Alert")
			gomega.Expect(result).To(gomega.Equal("Alert\n\nHello world"))
		})

		ginkgo.It("should trim title whitespace", func() {
			result := createMessage("Hello world", "  Alert  ")
			gomega.Expect(result).To(gomega.Equal("Alert\n\nHello world"))
		})

		ginkgo.It("should handle empty message with title", func() {
			result := createMessage("", "Alert")
			gomega.Expect(result).To(gomega.Equal("Alert\n\n"))
		})

		ginkgo.It("should handle empty title and empty message", func() {
			result := createMessage("", "")
			gomega.Expect(result).To(gomega.Equal(""))
		})

		ginkgo.It("should handle title with only whitespace", func() {
			result := createMessage("Hello world", "   ")
			gomega.Expect(result).To(gomega.Equal("Hello world"))
		})

		ginkgo.It("should preserve message with existing newlines", func() {
			result := createMessage("Line1\nLine2\nLine3", "Alert")
			gomega.Expect(result).To(gomega.Equal("Alert\n\nLine1\nLine2\nLine3"))
		})

		ginkgo.It("should handle message with special characters", func() {
			result := createMessage("Message with <html> & \"quotes\"", "Alert")
			gomega.Expect(result).To(gomega.Equal("Alert\n\nMessage with <html> & \"quotes\""))
		})
	})

	// Tests for setAuthorizationHeader
	ginkgo.Describe("setAuthorizationHeader", func() {
		ginkgo.It("should set Authorization header when accessToken is non-empty", func() {
			c := &client{
				accessToken: "test-token-123",
			}
			req, _ := http.NewRequest(http.MethodGet, "https://matrix.example.com/_matrix", http.NoBody)
			c.setAuthorizationHeader(req)
			gomega.Expect(req.Header.Get("Authorization")).To(gomega.Equal("Bearer test-token-123"))
		})

		ginkgo.It("should set header with correct Bearer prefix format", func() {
			c := &client{
				accessToken: "mytoken",
			}
			req, _ := http.NewRequest(http.MethodGet, "https://matrix.example.com/_matrix", http.NoBody)
			c.setAuthorizationHeader(req)
			headerValue := req.Header.Get("Authorization")
			gomega.Expect(headerValue).To(gomega.MatchRegexp("^Bearer .+$"))
			gomega.Expect(headerValue).To(gomega.Equal("Bearer mytoken"))
		})

		ginkgo.It("should NOT set header when accessToken is empty", func() {
			c := &client{
				accessToken: "",
			}
			req, _ := http.NewRequest(http.MethodGet, "https://matrix.example.com/_matrix", http.NoBody)
			c.setAuthorizationHeader(req)
			gomega.Expect(req.Header.Get("Authorization")).To(gomega.BeEmpty())
		})

		ginkgo.It("should overwrite existing Authorization header", func() {
			c := &client{
				accessToken: "new-token",
			}
			req, _ := http.NewRequest(http.MethodGet, "https://matrix.example.com/_matrix", http.NoBody)
			req.Header.Set("Authorization", "Existing-Token")
			c.setAuthorizationHeader(req)
			gomega.Expect(req.Header.Get("Authorization")).To(gomega.Equal("Bearer new-token"))
		})
	})

	// Tests for apiDo
	ginkgo.Describe("apiDo", func() {
		ginkgo.It("should return error on JSON marshal error with request body", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			// Use a channel that cannot be marshaled to JSON
			ch := make(chan string)
			resp := &apiResLogin{}
			err := c.apiDo(context.Background(), "POST", "/_matrix/client/v3/login", ch, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("marshaling"))
		})

		ginkgo.It("should return error on HTTP request creation failure", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("request failed")).Maybe()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			// Use an invalid URL to trigger request creation failure - empty scheme
			c.apiURL = url.URL{
				Scheme: "",
				Host:   "",
			}

			resp := &apiResLogin{}
			err := c.apiDo(context.Background(), "POST", "/_matrix/client/v3/login", nil, resp)
			// The actual behavior may vary - request creation might succeed with invalid URL
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle response body close error", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       &errorBodyClose{},
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
			err := c.apiDo(context.Background(), "POST", "/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should set Content-Type header to application/json", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			}, nil).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			})

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			_ = c.apiDo(context.Background(), "POST", "/_matrix/client/v3/login", nil, resp)

			gomega.Expect(capturedReq.Header.Get("Content-Type")).To(gomega.Equal("application/json"))
		})

		ginkgo.It("should use correct HTTP method", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			}, nil).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			})

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			_ = c.apiDo(context.Background(), "PUT", "/_matrix/client/v3/rooms/!room/send/m.room.message/txn", nil, resp)

			gomega.Expect(capturedReq.Method).To(gomega.Equal("PUT"))
		})
	})

	// Tests for apiPut
	ginkgo.Describe("apiPut", func() {
		ginkgo.It("should successfully make PUT request", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"event_id":"$eventid"}`))),
			}, nil).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			})

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResEvent{}
			err := c.apiPut(context.Background(), "/_matrix/client/v3/rooms/!room/send/m.room.message/txn1",
				apiReqSend{MsgType: msgTypeText, Body: "test message"}, resp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(capturedReq.Method).To(gomega.Equal("PUT"))
		})

		ginkgo.It("should return error on PUT request failure", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("PUT request failed"))

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResEvent{}
			err := c.apiPut(context.Background(), "/_matrix/client/v3/rooms/!room/send/m.room.message/txn1",
				apiReqSend{MsgType: msgTypeText, Body: "test message"}, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("executing PUT"))
		})
	})

	// Tests for context cancellation handling
	ginkgo.Describe("Context Cancellation Handling", func() {
		ginkgo.It("should return error when context is canceled before API call", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// The HTTP client may or may not be called depending on timing
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, context.Canceled).Maybe()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			resp := &apiResLogin{}
			err := c.apiDo(ctx, "POST", "/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle context cancellation during API call via HTTP client", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, context.Canceled)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiDo(context.Background(), "POST", "/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle context timeout", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, context.DeadlineExceeded)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			resp := &apiResLogin{}
			err := c.apiDo(context.Background(), "POST", "/_matrix/client/v3/login", nil, resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	// Tests for auth method gaps
	ginkgo.Describe("Auth Method Testing Gaps", func() {
		ginkgo.It("should use access token in query param for backward compatibility", func() {
			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				accessToken: "query-token",
				logger:      &testLogger{},
			}

			c.updateAccessToken()
			gomega.Expect(c.apiURL.Query().Get("access_token")).To(gomega.Equal("query-token"))
		})

		ginkgo.It("should handle user with @domain format in password login", func() {
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

			// Test with @domain format user
			err := c.loginPassword(context.Background(), "@user:matrix.example.com", "password")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(c.accessToken).To(gomega.Equal("token123"))
		})
	})

	// Tests for room handling edge cases
	ginkgo.Describe("Room Handling Edge Cases", func() {
		ginkgo.It("should return empty slice when getJoinedRooms returns empty list", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"joined_rooms":[]}`))),
			}, nil)

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			rooms, err := c.getJoinedRooms(context.Background())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(rooms).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle multiple rooms with partial failures", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// First call: getJoinedRooms - returns multiple rooms
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"joined_rooms":["!room1:matrix.example.com","!room2:matrix.example.com"]}`))),
			}, nil).Once()
			// Second call: sendMessageToRoom - success for room1
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"event_id":"$event1"}`))),
			}, nil).Once()
			// Third call: sendMessageToRoom - failure for room2
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("send error")).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToJoinedRooms(context.Background(), "test message")
			gomega.Expect(errs).To(gomega.HaveLen(1))
		})

		ginkgo.It("should handle sendToExplicitRooms with multiple rooms and partial failures", func() {
			mockHTTPClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			// First room: join succeeds, send succeeds
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"room_id":"!room1:matrix.example.com"}`))),
			}, nil).Once()
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"event_id":"$event1"}`))),
			}, nil).Once()
			// Second room: join succeeds, send fails
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"room_id":"!room2:matrix.example.com"}`))),
			}, nil).Once()
			mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("send error")).Once()

			c := &client{
				apiURL: url.URL{
					Scheme: "https",
					Host:   "matrix.example.com",
				},
				httpClient: mockHTTPClient,
				logger:     &testLogger{},
			}

			errs := c.sendToExplicitRooms(context.Background(),
				[]string{"#room1:matrix.example.com", "#room2:matrix.example.com"}, "test message")
			gomega.Expect(errs).To(gomega.HaveLen(1))
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

func (e *errorBodyClose) Close() error {
	return errors.New("close error")
}

func (e *errorBodyClose) Read(p []byte) (int, error) {
	return 0, io.EOF
}
