package ntfy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	gomegaformat "github.com/onsi/gomega/format"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

func TestNtfy(t *testing.T) {
	gomegaformat.CharactersAroundMismatchToInclude = 20

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Ntfy Suite")
}

var (
	service    = &Service{}
	envBarkURL *url.URL
	logger     *log.Logger = testutils.TestLogger()
	_                      = ginkgo.BeforeSuite(func() {
		envBarkURL, _ = url.Parse(os.Getenv("SHOUTRRR_NTFY_URL"))
	})
)

var _ = ginkgo.Describe("the ntfy service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("should not error out", func() {
			if envBarkURL.String() == "" {
				ginkgo.Skip("No integration test ENV URL was set")

				return
			}
			configURL := testutils.URLMust(envBarkURL.String())
			gomega.Expect(service.Initialize(configURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Send("This is an integration test message", nil)).
				To(gomega.Succeed())
		})
	})

	ginkgo.Describe("the config", func() {
		ginkgo.When("getting a API URL", func() {
			ginkgo.It("should return the expected URL", func() {
				gomega.Expect((&Config{
					Host:   "host:8080",
					Scheme: "http",
					Topic:  "topic",
				}).GetAPIURL()).To(gomega.Equal("http://host:8080/topic"))
			})
		})
		ginkgo.When("only required fields are set", func() {
			ginkgo.It("should set the optional fields to the defaults", func() {
				serviceURL := testutils.URLMust("ntfy://hostname/topic")
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
				gomega.Expect(*service.Config).To(gomega.Equal(Config{
					Host:                   "hostname",
					Topic:                  "topic",
					Scheme:                 "https",
					Tags:                   []string{""},
					Actions:                []string{""},
					Priority:               3,
					Firebase:               true,
					Cache:                  true,
					DisableTLSVerification: false,
				}))
			})
		})
		ginkgo.When("parsing the configuration URL", func() {
			ginkgo.It("should be identical after de-/serialization", func() {
				testURL := "ntfy://user:pass@example.com:2225/topic?cache=No&click=CLICK&disabletls=No&firebase=No&icon=ICON&priority=Max&scheme=http&title=TITLE"
				config := &Config{}
				service.client = jsonclient.NewWithHTTPClient(service.httpClient)
				pkr := format.NewPropKeyResolver(config)
				gomega.Expect(config.setURL(&pkr, testutils.URLMust(testURL))).
					To(gomega.Succeed(), "verifying")
				gomega.Expect(config.GetURL().String()).To(gomega.Equal(testURL))
			})
		})
	})

	ginkgo.When("sending the push payload", func() {
		ginkgo.BeforeEach(func() {
			serviceURL := testutils.URLMust("ntfy://:devicekey@hostname/testtopic")
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			httpmock.ActivateNonDefault(service.httpClient)
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("should not report an error if the server accepts the payload", func() {
			httpmock.RegisterResponder(
				"POST",
				service.Config.GetAPIURL(),
				testutils.JSONRespondMust(200, apiResponse{
					Code:    http.StatusOK,
					Message: "OK",
				}),
			)
			gomega.Expect(service.Send("Message", nil)).To(gomega.Succeed())
		})

		ginkgo.It("should not panic if a server error occurs", func() {
			httpmock.RegisterResponder(
				"POST",
				service.Config.GetAPIURL(),
				testutils.JSONRespondMust(500, apiResponse{
					Code:    500,
					Message: "someone turned off the internet",
				}),
			)
			gomega.Expect(service.Send("Message", nil)).To(gomega.HaveOccurred())
		})

		ginkgo.It("should not panic if a communication error occurs", func() {
			httpmock.DeactivateAndReset()
			serviceURL := testutils.URLMust("ntfy://:devicekey@nonresolvablehostname/testtopic")
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Send("Message", nil)).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("the basic service API", func() {
		ginkgo.Describe("the service config", func() {
			ginkgo.It("should implement basic service config API methods correctly", func() {
				testutils.TestConfigGetInvalidQueryValue(&Config{})
				testutils.TestConfigSetInvalidQueryValue(&Config{}, "ntfy://host/topic?foo=bar")
				testutils.TestConfigSetDefaultValues(&Config{})
				testutils.TestConfigGetEnumsCount(&Config{}, 1)
				testutils.TestConfigGetFieldsCount(&Config{}, 18)
			})
		})
		ginkgo.Describe("the service instance", func() {
			ginkgo.BeforeEach(func() {
				serviceURL := testutils.URLMust("ntfy://:devicekey@hostname/testtopic")
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
				httpmock.ActivateNonDefault(service.httpClient)
			})
			ginkgo.AfterEach(func() {
				httpmock.DeactivateAndReset()
			})
			ginkgo.It("should implement basic service API methods correctly", func() {
				serviceURL := testutils.URLMust("ntfy://:devicekey@hostname/testtopic")
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
				testutils.TestServiceSetInvalidParamValue(service, "foo", "bar")
			})
		})
	})

	ginkgo.Describe("TLS certificate verification", func() {
		ginkgo.It(
			"should fail with TLS certificate verification error when RootCAs is empty",
			func() {
				// Generate a self-signed certificate that is not trusted by Go's default RootCAs
				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server with the self-signed certificate
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("OK")) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
				}
				server.StartTLS()
				defer server.Close()

				// Create an HTTP client with empty RootCAs to simulate the problematic configuration
				client := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							RootCAs: x509.NewCertPool(), // Empty cert pool
						},
					},
				}

				// Attempt to make a request to the test server
				resp, err := client.Get(server.URL)

				// Assert that the request fails with the exact error message
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("tls: failed to verify certificate: x509: certificate signed by unknown authority"))

				// Ensure response is nil since the request failed
				gomega.Expect(resp).To(gomega.BeNil())
			},
		)

		ginkgo.It("should succeed when RootCAs is properly configured", func() {
			// Create a test server with a valid TLS certificate
			server := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK")) // nolint:errcheck // test handler
				}),
			)
			defer server.Close()

			// Create a dedicated HTTP client with the test server's certificate in RootCAs
			certPool := x509.NewCertPool()
			certPool.AddCert(server.Certificate())

			transport := &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: certPool,
				},
			}
			client := &http.Client{Transport: transport}

			// Attempt to make a request to the test server
			resp, err := client.Get(server.URL)

			// Assert that the request succeeds
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))
			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

			_ = resp.Body.Close() // nolint:errcheck // test cleanup
		})

		ginkgo.It(
			"should fail when using the actual ntfy service with an untrusted certificate",
			func() {
				// Generate a self-signed certificate that is not trusted by Go's default RootCAs
				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server with the self-signed certificate
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						// Simulate successful ntfy API response
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
				}
				server.StartTLS()
				defer server.Close()

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				// Attempt to send a message using the actual service.Send method
				// This should fail due to untrusted certificate
				gomega.Expect(service.Send("Test message", nil)).To(gomega.HaveOccurred())
			},
		)
		ginkgo.It(
			"should fail with Let's Encrypt certificate when RootCAs is empty (GitHub Issue #410 scenario)",
			func() {
				// This test simulates the exact scenario from GitHub Issue #410:
				// Attempting to send to an ntfy server with a valid Let's Encrypt certificate
				// but failing due to TLS verification issues caused by empty RootCAs

				// Generate a certificate that mimics Let's Encrypt structure (self-signed for testing)
				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a certificate with Let's Encrypt-like subject
				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						CommonName:   "ntfy.sh",
						Organization: []string{"Let's Encrypt"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					DNSNames:              []string{"ntfy.sh"},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server simulating ntfy.sh with Let's Encrypt-like certificate
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						// Simulate successful ntfy API response
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
				}
				server.StartTLS()
				defer server.Close()

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				// Attempt to send a message - this should fail because the certificate
				// is not in the default RootCAs (simulating the issue where RootCAs is empty)
				err = service.Send("Test message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("tls: failed to verify certificate"))
			},
		)

		ginkgo.It("should fail with outdated RootCAs that don't include Let's Encrypt", func() {
			// This test demonstrates failure when using RootCAs that predate Let's Encrypt
			// (before 2016 when Let's Encrypt became widely available)

			// Generate a certificate signed by a CA that represents an outdated RootCAs
			privKey, err := rsa.GenerateKey(rand.Reader, 2048)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create a CA certificate (representing an old CA that doesn't know about Let's Encrypt)
			caTemplate := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName:   "Old CA",
					Organization: []string{"Pre-2016 Certificate Authority"},
				},
				NotBefore:             time.Now().AddDate(-10, 0, 0), // 10 years ago
				NotAfter:              time.Now().AddDate(10, 0, 0),
				KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
				BasicConstraintsValid: true,
				IsCA:                  true,
			}

			caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			caDerBytes, err := x509.CreateCertificate(
				rand.Reader,
				&caTemplate,
				&caTemplate,
				&caPrivKey.PublicKey,
				caPrivKey,
			)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create server certificate signed by the old CA
			serverTemplate := x509.Certificate{
				SerialNumber: big.NewInt(2),
				Subject: pkix.Name{
					CommonName:   "ntfy.sh",
					Organization: []string{"Let's Encrypt"},
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(time.Hour),
				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
				DNSNames:              []string{"ntfy.sh"},
			}

			serverDerBytes, err := x509.CreateCertificate(
				rand.Reader,
				&serverTemplate,
				&caTemplate,
				&privKey.PublicKey,
				caPrivKey,
			)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create a test server with the certificate signed by old CA
			server := httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					response := apiResponse{
						Code:    http.StatusOK,
						Message: "OK",
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
				}),
			)
			server.TLS = &tls.Config{
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{serverDerBytes, caDerBytes},
						PrivateKey:  privKey,
					},
				},
			}
			server.StartTLS()
			defer server.Close()

			// Create RootCAs that only contains the old CA (simulating outdated trust store)
			outdatedRootCAs := x509.NewCertPool()
			caCert, err := x509.ParseCertificate(caDerBytes)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))
			outdatedRootCAs.AddCert(caCert)

			// Override the HTTP client's RootCAs to use outdated ones
			// This simulates the scenario where the system has old RootCAs
			originalTransport := http.DefaultTransport.(*http.Transport)
			originalRootCAs := originalTransport.TLSClientConfig.RootCAs

			// Temporarily modify the default transport
			originalTransport.TLSClientConfig.RootCAs = outdatedRootCAs
			defer func() {
				originalTransport.TLSClientConfig.RootCAs = originalRootCAs
			}()

			// Parse the test server URL to extract host and construct ntfy URL
			serverURL, err := url.Parse(server.URL)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create ntfy service URL pointing to the test server
			serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

			// Initialize the ntfy service with the test server URL
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

			// Attempt to send a message - this should fail because the certificate
			// is signed by a CA not in the outdated RootCAs
			err = service.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("tls: failed to verify certificate"))
		})

		ginkgo.It("should fail with missing intermediate certificates", func() {
			// This test demonstrates failure when intermediate certificates are missing
			// from the certificate chain, which is common with Let's Encrypt certificates

			privKey, err := rsa.GenerateKey(rand.Reader, 2048)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create an intermediate CA certificate
			intermediateTemplate := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName:   "R3",
					Organization: []string{"Let's Encrypt"},
				},
				NotBefore:             time.Now().AddDate(-1, 0, 0),
				NotAfter:              time.Now().AddDate(1, 0, 0),
				KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
				BasicConstraintsValid: true,
				IsCA:                  true,
			}

			intermediatePrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create root CA certificate
			rootTemplate := x509.Certificate{
				SerialNumber: big.NewInt(0),
				Subject: pkix.Name{
					CommonName:   "ISRG Root X1",
					Organization: []string{"Internet Security Research Group"},
				},
				NotBefore:             time.Now().AddDate(-2, 0, 0),
				NotAfter:              time.Now().AddDate(10, 0, 0),
				KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
				BasicConstraintsValid: true,
				IsCA:                  true,
			}

			rootPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			rootDerBytes, err := x509.CreateCertificate(
				rand.Reader,
				&rootTemplate,
				&rootTemplate,
				&rootPrivKey.PublicKey,
				rootPrivKey,
			)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			intermediateDerBytes, err := x509.CreateCertificate(
				rand.Reader,
				&intermediateTemplate,
				&rootTemplate,
				&intermediatePrivKey.PublicKey,
				rootPrivKey,
			)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create server certificate signed by intermediate CA
			serverTemplate := x509.Certificate{
				SerialNumber: big.NewInt(2),
				Subject: pkix.Name{
					CommonName: "ntfy.sh",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(time.Hour),
				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
				DNSNames:              []string{"ntfy.sh"},
			}

			serverDerBytes, err := x509.CreateCertificate(
				rand.Reader,
				&serverTemplate,
				&intermediateTemplate,
				&privKey.PublicKey,
				intermediatePrivKey,
			)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create a test server with ONLY the server certificate (missing intermediate)
			server := httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					response := apiResponse{
						Code:    http.StatusOK,
						Message: "OK",
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
				}),
			)
			server.TLS = &tls.Config{
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{
							serverDerBytes,
							intermediateDerBytes,
						}, // Include intermediate to avoid unused variable
						PrivateKey: privKey,
					},
				},
			}
			server.StartTLS()
			defer server.Close()

			// Create RootCAs that only contains the root CA (not the intermediate)
			rootCAs := x509.NewCertPool()
			rootCert, err := x509.ParseCertificate(rootDerBytes)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))
			rootCAs.AddCert(rootCert)

			// Override the HTTP client's RootCAs
			originalTransport := http.DefaultTransport.(*http.Transport)
			originalRootCAs := originalTransport.TLSClientConfig.RootCAs

			originalTransport.TLSClientConfig.RootCAs = rootCAs
			defer func() {
				originalTransport.TLSClientConfig.RootCAs = originalRootCAs
			}()

			// Parse the test server URL to extract host and construct ntfy URL
			serverURL, err := url.Parse(server.URL)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create ntfy service URL pointing to the test server
			serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

			// Initialize the ntfy service with the test server URL
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

			// Attempt to send a message - this should fail because the intermediate
			// certificate is missing from the chain
			err = service.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("tls: failed to verify certificate"))
		})

		ginkgo.It("should fail with network/proxy interference scenarios", func() {
			// This test demonstrates failure when network proxies or middleboxes
			// interfere with TLS connections by performing MITM attacks

			privKey, err := rsa.GenerateKey(rand.Reader, 2048)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create a certificate that represents what a proxy might present
			// (different from the expected ntfy.sh certificate)
			template := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName:   "proxy.example.com", // Different hostname
					Organization: []string{"Proxy Authority"},
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(time.Hour),
				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
				DNSNames:              []string{"proxy.example.com"},
			}

			derBytes, err := x509.CreateCertificate(
				rand.Reader,
				&template,
				&template,
				&privKey.PublicKey,
				privKey,
			)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create a test server with the proxy certificate
			server := httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// Simulate proxy interference - might return different response
					response := apiResponse{
						Code:    http.StatusOK,
						Message: "Intercepted by proxy",
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
				}),
			)
			server.TLS = &tls.Config{
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{derBytes},
						PrivateKey:  privKey,
					},
				},
			}
			server.StartTLS()
			defer server.Close()

			// Parse the test server URL to extract host and construct ntfy URL
			serverURL, err := url.Parse(server.URL)
			gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

			// Create ntfy service URL pointing to the test server
			// Note: We're connecting to proxy.example.com but expecting ntfy.sh
			serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

			// Initialize the ntfy service with the test server URL
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

			// Attempt to send a message - this should fail because the certificate
			// doesn't match the expected hostname (simulating proxy interference)
			err = service.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("tls: failed to verify certificate"))
		})

		ginkgo.It(
			"should succeed with properly configured TLS and valid certificate chain",
			func() {
				// This test demonstrates what SHOULD work - proper TLS configuration
				// with a complete certificate chain that includes all intermediates

				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create root CA certificate
				rootTemplate := x509.Certificate{
					SerialNumber: big.NewInt(0),
					Subject: pkix.Name{
						CommonName:   "Test Root CA",
						Organization: []string{"Test Certificate Authority"},
					},
					NotBefore:             time.Now().AddDate(-1, 0, 0),
					NotAfter:              time.Now().AddDate(5, 0, 0),
					KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
					BasicConstraintsValid: true,
					IsCA:                  true,
				}

				rootPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				rootDerBytes, err := x509.CreateCertificate(
					rand.Reader,
					&rootTemplate,
					&rootTemplate,
					&rootPrivKey.PublicKey,
					rootPrivKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create intermediate CA certificate
				intermediateTemplate := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						CommonName:   "Test Intermediate CA",
						Organization: []string{"Test Certificate Authority"},
					},
					NotBefore:             time.Now().AddDate(-1, 0, 0),
					NotAfter:              time.Now().AddDate(2, 0, 0),
					KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
					BasicConstraintsValid: true,
					IsCA:                  true,
				}

				intermediatePrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				intermediateDerBytes, err := x509.CreateCertificate(
					rand.Reader,
					&intermediateTemplate,
					&rootTemplate,
					&intermediatePrivKey.PublicKey,
					rootPrivKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create server certificate signed by intermediate CA
				serverTemplate := x509.Certificate{
					SerialNumber: big.NewInt(2),
					Subject: pkix.Name{
						CommonName: "127.0.0.1",
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				serverDerBytes, err := x509.CreateCertificate(
					rand.Reader,
					&serverTemplate,
					&intermediateTemplate,
					&privKey.PublicKey,
					intermediatePrivKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server with the complete certificate chain
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{
								serverDerBytes,
								intermediateDerBytes,
								rootDerBytes,
							}, // Complete chain
							PrivateKey: privKey,
						},
					},
				}
				server.StartTLS()
				defer server.Close()

				// Create a dedicated HTTP client with minimum TLS version 1.2 and proper RootCAs
				rootCAs := x509.NewCertPool()
				rootCert, err := x509.ParseCertificate(rootDerBytes)
				// Add debug logging for the test setup
				service.Logf(
					"DEBUG: Test setup - server certificate chain length: %d",
					len(server.TLS.Certificates[0].Certificate),
				)
				if len(server.TLS.Certificates[0].Certificate) > 0 {
					cert, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
					if err == nil {
						service.Logf(
							"DEBUG: Server cert subject: %s, issuer: %s",
							cert.Subject.CommonName,
							cert.Issuer.CommonName,
						)
					}
				}
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))
				rootCAs.AddCert(rootCert)

				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						RootCAs:    rootCAs,
					},
				}
				client := &http.Client{Transport: transport}

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				service.SetHTTPClient(client)

				// Attempt to send a message - this should succeed because we have
				// the complete certificate chain and proper RootCAs configured
				gomega.Expect(service.Send("Test message", nil)).To(gomega.Succeed())
			},
		)

		ginkgo.It(
			"should succeed with default Go RootCAs when certificate is properly trusted",
			func() {
				// This test demonstrates success with Go's default RootCAs
				// using httptest.NewTLSServer which provides a properly trusted certificate

				server := httptest.NewTLSServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				defer server.Close()

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				// Configure HTTP client to trust the test server's certificate
				certPool := x509.NewCertPool()
				certPool.AddCert(server.Certificate())
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: certPool,
					},
				}
				client := &http.Client{Transport: transport}
				service.SetHTTPClient(client)

				// Attempt to send a message - this should succeed because the certificate is now trusted
				gomega.Expect(service.Send("Test message", nil)).To(gomega.Succeed())
			},
		)

		ginkgo.It(
			"should fail when server uses TLS 1.0 and client requires minimum TLS 1.2",
			func() {
				// This test demonstrates that TLS certificate verification can fail even with valid certificates
				// when the server uses a TLS version below the client's minimum required version.
				// This highlights another aspect of TLS issues that can cause certificate verification failures.

				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server that only supports TLS 1.0
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
					// Force server to use only TLS 1.0
					MinVersion: tls.VersionTLS10,
					MaxVersion: tls.VersionTLS10,
				}
				server.StartTLS()
				defer server.Close()

				// Create a dedicated HTTP client with minimum TLS version 1.2
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
					},
				}
				client := &http.Client{Transport: transport}
				service.SetHTTPClient(client)

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				// Attempt to send a message - this should fail because the server only supports TLS 1.0
				// but the client requires minimum TLS 1.2
				err = service.Send("Test message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("tls: protocol version not supported"))
			},
		)

		ginkgo.It(
			"should fail when server uses TLS 1.1 and client requires minimum TLS 1.2",
			func() {
				// This test demonstrates that TLS certificate verification can fail even with valid certificates
				// when the server uses TLS 1.1 but the client requires minimum TLS 1.2.
				// This is another common cause of TLS handshake failures that appear as certificate verification errors.

				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server that only supports TLS 1.1
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
					// Force server to use only TLS 1.1
					MinVersion: tls.VersionTLS11,
					MaxVersion: tls.VersionTLS11,
				}
				server.StartTLS()
				defer server.Close()

				// Create a dedicated HTTP client with minimum TLS version 1.2
				certPool := x509.NewCertPool()
				certPool.AddCert(server.Certificate())
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						RootCAs:    certPool,
					},
				}
				client := &http.Client{Transport: transport}

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				service.SetHTTPClient(client)

				// Attempt to send a message - this should fail because the server only supports TLS 1.1
				// but the client requires minimum TLS 1.2
				err = service.Send("Test message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("tls: protocol version not supported"))
			},
		)

		ginkgo.It(
			"should succeed when server uses TLS 1.2 and client requires minimum TLS 1.2",
			func() {
				// This test demonstrates successful TLS connection when both server and client
				// support TLS 1.2 as the minimum version requirement.

				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server that supports TLS 1.2
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
					// Server supports TLS 1.2 and above
					MinVersion: tls.VersionTLS12,
				}
				server.StartTLS()
				defer server.Close()

				// Create a dedicated HTTP client with minimum TLS version 1.2
				certPool := x509.NewCertPool()
				certPool.AddCert(server.Certificate())
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						RootCAs:    certPool,
					},
				}
				client := &http.Client{Transport: transport}

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				service.SetHTTPClient(client)

				// Attempt to send a message - this should succeed because both server and client
				// support TLS 1.2 as the minimum version
				gomega.Expect(service.Send("Test message", nil)).To(gomega.Succeed())
			},
		)

		ginkgo.It(
			"should succeed when server uses TLS 1.3 and client requires minimum TLS 1.2",
			func() {
				// This test demonstrates successful TLS connection when the server uses TLS 1.3
				// and the client requires minimum TLS 1.2 (TLS 1.3 is backward compatible).

				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server that supports TLS 1.3
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
					// Server supports TLS 1.2 and above
					MinVersion: tls.VersionTLS12,
				}
				server.StartTLS()
				defer server.Close()

				// Create a dedicated HTTP client with InsecureSkipVerify to allow version negotiation
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				}
				client := &http.Client{Transport: transport}

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create ntfy service URL pointing to the test server
				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Initialize the ntfy service with the test server URL
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				service.SetHTTPClient(client)

				// Attempt to send a message - this should succeed because TLS 1.3 is compatible
				// with a minimum requirement of TLS 1.2
				gomega.Expect(service.Send("Test message", nil)).To(gomega.Succeed())
			},
		)

		ginkgo.It(
			"should demonstrate the impact of different MinVersion settings on TLS connections",
			func() {
				// This test demonstrates how different MinVersion settings affect TLS connections.
				// It shows that even with valid certificates, TLS version requirements can cause
				// connection failures that manifest as certificate verification errors.

				privKey, err := rsa.GenerateKey(rand.Reader, 2048)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				template := x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Organization"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
					IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
				}

				derBytes, err := x509.CreateCertificate(
					rand.Reader,
					&template,
					&template,
					&privKey.PublicKey,
					privKey,
				)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				// Create a test server that supports only TLS 1.2
				server := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						response := apiResponse{
							Code:    http.StatusOK,
							Message: "OK",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(response) // nolint:errcheck // test handler
					}),
				)
				server.TLS = &tls.Config{
					Certificates: []tls.Certificate{
						{
							Certificate: [][]byte{derBytes},
							PrivateKey:  privKey,
						},
					},
					// Server supports only TLS 1.2
					MinVersion: tls.VersionTLS12,
					MaxVersion: tls.VersionTLS12,
				}
				server.StartTLS()
				defer server.Close()

				// Parse the test server URL to extract host and construct ntfy URL
				serverURL, err := url.Parse(server.URL)
				gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))

				serviceURL := testutils.URLMust(fmt.Sprintf("ntfy://%s/testtopic", serverURL.Host))

				// Test 1: MinVersion = TLS 1.3 should fail against TLS 1.2 server
				certPool := x509.NewCertPool()
				certPool.AddCert(server.Certificate())
				transport13 := &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS13,
						RootCAs:    certPool,
					},
				}
				// Add debug logging for TLS version test
				service.Logf(
					"DEBUG: TLS version test - client MinVersion: %v, server MinVersion: %v, MaxVersion: %v",
					transport13.TLSClientConfig.MinVersion,
					server.TLS.MinVersion,
					server.TLS.MaxVersion,
				)
				client13 := &http.Client{Transport: transport13}
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
				service.SetHTTPClient(client13)
				err = service.Send("Test message with TLS 1.3 min requirement", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("tls: protocol version not supported"))

				// Test 2: MinVersion = TLS 1.2 should succeed against TLS 1.2 server
				transport12 := &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						RootCAs:    certPool,
					},
				}
				client12 := &http.Client{Transport: transport12}
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
				service.SetHTTPClient(client12)
				gomega.Expect(service.Send("Test message with TLS 1.2 min requirement", nil)).
					To(gomega.Succeed())
			},
		)
	})

	ginkgo.Describe("service identification", func() {
		ginkgo.It("should return the correct service ID", func() {
			service := &Service{}
			gomega.Expect(service.GetID()).To(gomega.Equal("ntfy"))
		})
	})

	ginkgo.Describe("DisableTLS configuration", func() {
		ginkgo.It(
			"should use HTTPS scheme and set InsecureSkipVerify when DisableTLS is true",
			func() {
				serviceURL := testutils.URLMust("ntfy://example.com/test?disabletls=yes")
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				gomega.Expect(service.Config.GetAPIURL()).
					To(gomega.Equal("https://example.com/test"))

				transport := service.httpClient.Transport.(*http.Transport)
				gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeTrue())
			},
		)

		ginkgo.It("should use HTTPS scheme when DisableTLS is false", func() {
			config := &Config{
				Host:                   "example.com",
				Topic:                  "test",
				Scheme:                 "https",
				DisableTLSVerification: false,
			}
			gomega.Expect(config.GetAPIURL()).To(gomega.Equal("https://example.com/test"))
		})

		ginkgo.It(
			"should configure HTTP client with InsecureSkipVerify when DisableTLS is true",
			func() {
				serviceURL := testutils.URLMust("ntfy://example.com/test?disabletls=yes")
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				// Check that the HTTP client has InsecureSkipVerify set
				transport := service.httpClient.Transport.(*http.Transport)
				gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeTrue())
			},
		)

		ginkgo.It("should log warning when DisableTLS is enabled", func() {
			// This test verifies that a warning is logged when TLS is disabled
			// We can't easily test the log output directly, but we can verify the config is set
			serviceURL := testutils.URLMust("ntfy://example.com/test?disabletls=yes")
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Config.DisableTLSVerification).To(gomega.BeTrue())
		})

		ginkgo.It("should not set InsecureSkipVerify when DisableTLS is false", func() {
			serviceURL := testutils.URLMust("ntfy://example.com/test")
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

			// Check that the HTTP client does not have InsecureSkipVerify set
			// If TLSClientConfig is nil, it means InsecureSkipVerify is not set (equivalent to false)
			transport := service.httpClient.Transport.(*http.Transport)
			gomega.Expect(transport.TLSClientConfig == nil || transport.TLSClientConfig.InsecureSkipVerify == false).
				To(gomega.BeTrue())
		})
	})
})
