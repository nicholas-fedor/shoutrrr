package pushover_test

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/pushover"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	hookURL                = "https://api.pushover.net/1/messages.json"
	testEncryptionKey      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testEncryptionKeyMixed = "0123456789ABCDEF0123456789abcdef0123456789ABCDEF0123456789abcdef"
)

var (
	service        *pushover.Service
	config         *pushover.Config
	keyResolver    format.PropKeyResolver
	envPushoverURL *url.URL
	logger         *log.Logger
	_              = ginkgo.BeforeSuite(func() {
		service = &pushover.Service{}
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
		envPushoverURL, _ = url.Parse(os.Getenv("SHOUTRRR_PUSHOVER_URL"))
	})
)

var _ = ginkgo.Describe("the pushover service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("should work", func() {
			if envPushoverURL.String() == "" {
				return
			}

			serviceURL, _ := url.Parse(envPushoverURL.String())
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("this is an integration test", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("returns the correct service identifier", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("pushover"))
		})
	})
})

var _ = ginkgo.Describe("the pushover config", func() {
	ginkgo.BeforeEach(func() {
		config = &pushover.Config{}
		keyResolver = format.NewPropKeyResolver(config)
	})
	ginkgo.When("updating it using an url", func() {
		ginkgo.It("should update the username using the host part of the url", func() {
			url := createURL("simme", "dummy")
			err := config.SetURL(url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.User).To(gomega.Equal("simme"))
		})
		ginkgo.It("should update the token using the password part of the url", func() {
			url := createURL("dummy", "TestToken")
			err := config.SetURL(url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Token).To(gomega.Equal("TestToken"))
		})
		ginkgo.It("should error if supplied with an empty username", func() {
			url := createURL("", "token")
			expectErrorGivenURL(pushover.ErrUserMissing, url)
		})
		ginkgo.It("should error if supplied with an empty token", func() {
			url := createURL("user", "")
			expectErrorGivenURL(pushover.ErrTokenMissing, url)
		})
	})
	ginkgo.When("getting the current config", func() {
		ginkgo.It("should return the config that is currently set as an url", func() {
			config.User = "simme"
			config.Token = "test-token"

			url := config.GetURL()
			password, _ := url.User.Password()
			gomega.Expect(url.Host).To(gomega.Equal(config.User))
			gomega.Expect(password).To(gomega.Equal(config.Token))
			gomega.Expect(url.Scheme).To(gomega.Equal("pushover"))
		})
	})
	ginkgo.When("setting a config key", func() {
		ginkgo.It("should split it by commas if the key is devices", func() {
			err := keyResolver.Set("devices", "a,b,c,d")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Devices).To(gomega.Equal([]string{"a", "b", "c", "d"}))
		})
		ginkgo.It("should update priority when a valid number is supplied", func() {
			err := keyResolver.Set("priority", "1")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Priority).To(gomega.Equal(int8(1)))
		})
		ginkgo.It("should update priority when a negative number is supplied", func() {
			gomega.Expect(keyResolver.Set("priority", "-1")).To(gomega.Succeed())
			gomega.Expect(config.Priority).To(gomega.BeEquivalentTo(-1))

			gomega.Expect(keyResolver.Set("priority", "-2")).To(gomega.Succeed())
			gomega.Expect(config.Priority).To(gomega.BeEquivalentTo(-2))
		})
		ginkgo.It("should update the title when it is supplied", func() {
			err := keyResolver.Set("title", "new title")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Title).To(gomega.Equal("new title"))
		})
		ginkgo.It("should return an error if priority is not a number", func() {
			err := keyResolver.Set("priority", "super-duper")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should return an error if the key is not recognized", func() {
			err := keyResolver.Set("devicey", "a,b,c,d")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should accept the encryption key alias", func() {
			err := keyResolver.Set("key", testEncryptionKey)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.EncryptionKey).To(gomega.Equal(testEncryptionKey))
		})
	})
	ginkgo.When("getting a config key", func() {
		ginkgo.It("should join it with commas if the key is devices", func() {
			config.Devices = []string{"a", "b", "c"}
			value, err := keyResolver.Get("devices")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(value).To(gomega.Equal("a,b,c"))
		})
		ginkgo.It("should return an error if the key is not recognized", func() {
			_, err := keyResolver.Get("devicey")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.When("listing the query fields", func() {
		ginkgo.It("should return the keys \"devices\",\"encryptionkey\",\"key\",\"priority\",\"title\"", func() {
			fields := keyResolver.QueryFields()
			gomega.Expect(fields).To(gomega.Equal([]string{
				"devices",
				"encryptionkey",
				"key",
				"priority",
				"title",
			}))
		})
	})

	ginkgo.When("configuring end-to-end encryption", func() {
		ginkgo.It("should accept a mixed-case 64-character hex key", func() {
			serviceURL, err := url.Parse(
				"pushover://:apptoken@usertoken?encryptionkey=" + testEncryptionKeyMixed,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.EncryptionKey).To(gomega.Equal(testEncryptionKeyMixed))
		})
		ginkgo.It("should reject a short encryption key", func() {
			serviceURL, err := url.Parse("pushover://:apptoken@usertoken?encryptionkey=0123456789abcdef")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(serviceURL)
			gomega.Expect(err).To(gomega.MatchError(pushover.ErrInvalidEncryptionKey))
		})
		ginkgo.It("should reject a long encryption key", func() {
			serviceURL, err := url.Parse(
				"pushover://:apptoken@usertoken?encryptionkey=" + testEncryptionKey + "ab",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(serviceURL)
			gomega.Expect(err).To(gomega.MatchError(pushover.ErrInvalidEncryptionKey))
		})
		ginkgo.It("should reject a non-hex encryption key", func() {
			serviceURL, err := url.Parse(
				"pushover://:apptoken@usertoken?encryptionkey=zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(serviceURL)
			gomega.Expect(err).To(gomega.MatchError(pushover.ErrInvalidEncryptionKey))
		})
		ginkgo.It("should round-trip the encryption key through GetURL", func() {
			config.User = "simme"
			config.Token = "test-token"
			config.EncryptionKey = testEncryptionKey

			serviceURL := config.GetURL()
			gomega.Expect(serviceURL.Query().Get("encryptionkey")).To(gomega.Equal(testEncryptionKey))
			gomega.Expect(serviceURL.Query().Get("key")).To(gomega.BeEmpty())

			roundTrip := &pushover.Config{}
			err := roundTrip.SetURL(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(roundTrip.EncryptionKey).To(gomega.Equal(testEncryptionKey))
		})
	})

	ginkgo.Describe("sending the payload", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})
		ginkgo.It("should not report an error if the server accepts the payload", func() {
			serviceURL, err := url.Parse("pushover://:apptoken@usertoken")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder("POST", hookURL, httpmock.NewStringResponder(200, ""))

			err = service.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should not panic if an error occurs when sending the payload", func() {
			serviceURL, err := url.Parse("pushover://:apptoken@usertoken")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				hookURL,
				httpmock.NewErrorResponder(errors.New("dummy error")),
			)

			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should send plaintext without an encrypted flag when no key is set", func() {
			serviceURL, err := url.Parse("pushover://:apptoken@usertoken?title=Plain+Title")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				hookURL,
				func(req *http.Request) (*http.Response, error) {
					gomega.Expect(req.ParseForm()).To(gomega.Succeed())
					gomega.Expect(req.Form.Get("encrypted")).To(gomega.BeEmpty())
					gomega.Expect(req.Form.Get("message")).To(gomega.Equal("Message"))
					gomega.Expect(req.Form.Get("title")).To(gomega.Equal("Plain Title"))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should encrypt message and title when an encryption key is set", func() {
			serviceURL, err := url.Parse(
				"pushover://:apptoken@usertoken?encryptionkey=" +
					testEncryptionKey +
					"&title=Secret+Title",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			keyBytes, err := hex.DecodeString(testEncryptionKey)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				hookURL,
				func(req *http.Request) (*http.Response, error) {
					gomega.Expect(req.ParseForm()).To(gomega.Succeed())
					gomega.Expect(req.Form.Get("encrypted")).To(gomega.Equal("1"))
					gomega.Expect(req.Form.Get("message")).NotTo(gomega.Equal("Secret message"))
					gomega.Expect(req.Form.Get("title")).NotTo(gomega.Equal("Secret Title"))

					message, decErr := decryptPushoverField(req.Form.Get("message"), keyBytes)
					gomega.Expect(decErr).NotTo(gomega.HaveOccurred())
					gomega.Expect(message).To(gomega.Equal("Secret message"))

					title, decErr := decryptPushoverField(req.Form.Get("title"), keyBytes)
					gomega.Expect(decErr).NotTo(gomega.HaveOccurred())
					gomega.Expect(title).To(gomega.Equal("Secret Title"))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Secret message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should fail to initialize when the encryption key is invalid", func() {
			serviceURL, err := url.Parse("pushover://:apptoken@usertoken?encryptionkey=short")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).To(gomega.MatchError(pushover.ErrInvalidEncryptionKey))
		})
		ginkgo.It("should not send when params supply an invalid encryption key", func() {
			serviceURL, err := url.Parse("pushover://:apptoken@usertoken")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				hookURL,
				func(*http.Request) (*http.Response, error) {
					ginkgo.Fail("HTTP request should not be sent")

					return nil, errors.New("unexpected HTTP request")
				},
			)

			err = service.Send("Message", &types.Params{"encryptionkey": "short"})
			gomega.Expect(err).To(gomega.MatchError(pushover.ErrInvalidEncryptionKey))
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(0))
		})
	})
})

func TestPushover(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Pushover Suite")
}

func createURL(username, token string) *url.URL {
	return &url.URL{
		User: url.UserPassword("Token", token),
		Host: username,
	}
}

func expectErrorGivenURL(expectedErr error, serviceURL *url.URL) {
	err := config.SetURL(serviceURL)
	gomega.Expect(err).To(gomega.HaveOccurred())
	gomega.Expect(err.Error()).To(gomega.Equal(expectedErr.Error()))
}

func decryptPushoverField(encoded string, key []byte) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	const hmacSize = sha256.Size
	if len(payload) < aes.BlockSize+hmacSize {
		return "", errors.New("ciphertext too short")
	}

	iv := payload[:aes.BlockSize]
	mac := payload[len(payload)-hmacSize:]
	ciphertext := payload[aes.BlockSize : len(payload)-hmacSize]

	expected := hmac.New(sha256.New, key)
	if _, err := expected.Write(iv); err != nil {
		return "", err
	}

	if _, err := expected.Write(ciphertext); err != nil {
		return "", err
	}

	if !hmac.Equal(mac, expected.Sum(nil)) {
		return "", errors.New("hmac mismatch")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext not a multiple of block size")
	}

	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)

	unpadded, err := pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", err
	}

	reader, err := gzip.NewReader(bytes.NewReader(unpadded))
	if err != nil {
		return "", err
	}

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	if err := reader.Close(); err != nil {
		return "", err
	}

	return string(decompressed), nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding")
	}

	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, errors.New("invalid padding")
	}

	for _, b := range data[len(data)-padLen:] {
		if b != byte(padLen) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padLen], nil
}
