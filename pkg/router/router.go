package router

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// DefaultTimeout is the default duration for service operation timeouts.
const DefaultTimeout = 10 * time.Second

var (
	ErrNoSenders              = errors.New("error sending message: no senders")
	ErrServiceTimeout         = errors.New("failed to send: timed out")
	ErrCustomURLsNotSupported = errors.New("custom URLs are not supported by service")
	ErrUnknownService         = errors.New("unknown service")
	ErrParseURLFailed         = errors.New("failed to parse URL")
	ErrSendFailed             = errors.New("failed to send message")
	ErrCustomURLConversion    = errors.New("failed to convert custom URL")
	ErrInitializeFailed       = errors.New("failed to initialize service")
)

// ServiceRouter is responsible for routing a message to a specific notification service using the notification URL.
type ServiceRouter struct {
	logger   types.StdLogger
	services []types.Service
	queue    []string
	Timeout  time.Duration
}

// New creates a new service router using the specified logger and service URLs.
func New(logger types.StdLogger, serviceURLs ...string) (*ServiceRouter, error) {
	router := ServiceRouter{
		logger:  logger,
		Timeout: DefaultTimeout,
	}

	for _, serviceURL := range serviceURLs {
		if err := router.AddService(serviceURL); err != nil {
			return nil, fmt.Errorf("error initializing router services: %w", err)
		}
	}

	return &router, nil
}

// AddService initializes the specified service from its URL, and adds it if no errors occur.
func (router *ServiceRouter) AddService(serviceURL string) error {
	service, err := router.initService(serviceURL)
	if err == nil {
		router.services = append(router.services, service)
	}

	return err
}

// Send sends the specified message using the routers underlying services.
func (router *ServiceRouter) Send(message string, params *types.Params) []error {
	if router == nil {
		return []error{ErrNoSenders}
	}

	serviceCount := len(router.services)
	errors := make([]error, serviceCount)
	results := router.SendAsync(message, params)

	for i := range router.services {
		errors[i] = <-results
	}

	return errors
}

// SendItems sends the specified message items using the routers underlying services.
func (router *ServiceRouter) SendItems(items []types.MessageItem, params types.Params) []error {
	if router == nil {
		return []error{ErrNoSenders}
	}

	// Fallback using old API for now
	message := strings.Builder{}
	for _, item := range items {
		message.WriteString(item.Text)
	}

	serviceCount := len(router.services)
	errors := make([]error, serviceCount)
	results := router.SendAsync(message.String(), &params)

	for i := range router.services {
		errors[i] = <-results
	}

	return errors
}

// SendAsync sends the specified message using the routers underlying services.
func (router *ServiceRouter) SendAsync(message string, params *types.Params) chan error {
	serviceCount := len(router.services)
	proxy := make(chan error, serviceCount)
	errors := make(chan error, serviceCount)

	if params == nil {
		params = &types.Params{}
	}

	for _, service := range router.services {
		go sendToService(service, proxy, router.Timeout, message, *params)
	}

	go func() {
		for range serviceCount {
			errors <- <-proxy
		}

		close(errors)
	}()

	return errors
}

func sendToService(
	service types.Service,
	results chan error,
	timeout time.Duration,
	message string,
	params types.Params,
) {
	result := make(chan error)

	serviceID := service.GetID()

	go func() { result <- service.Send(message, &params) }()

	select {
	case res := <-result:
		results <- res
	case <-time.After(timeout):
		results <- fmt.Errorf("%w: using %v", ErrServiceTimeout, serviceID)
	}
}

// Enqueue adds the message to an internal queue and sends it when Flush is invoked.
func (router *ServiceRouter) Enqueue(message string, v ...any) {
	if len(v) > 0 {
		message = fmt.Sprintf(message, v...)
	}

	router.queue = append(router.queue, message)
}

// Flush sends all messages that have been queued up as a combined message. This method should be deferred!
func (router *ServiceRouter) Flush(params *types.Params) {
	// Since this method is supposed to be deferred we just have to ignore errors
	_ = router.Send(strings.Join(router.queue, "\n"), params)
	router.queue = []string{}
}

// SetLogger sets the logger that the services will use to write progress logs.
func (router *ServiceRouter) SetLogger(logger types.StdLogger) {
	router.logger = logger
	for _, service := range router.services {
		service.SetLogger(logger)
	}
}

// ExtractServiceName from a notification URL.
func (router *ServiceRouter) ExtractServiceName(rawURL string) (string, *url.URL, error) {
	serviceURL, err := url.Parse(rawURL)
	if err != nil {
		return "", &url.URL{}, fmt.Errorf("%s: %w", rawURL, ErrParseURLFailed)
	}

	scheme := serviceURL.Scheme
	schemeParts := strings.Split(scheme, "+")

	if len(schemeParts) > 1 {
		scheme = schemeParts[0]
	}

	return scheme, serviceURL, nil
}

// Route a message to a specific notification service using the notification URL.
func (router *ServiceRouter) Route(rawURL string, message string) error {
	service, err := router.Locate(rawURL)
	if err != nil {
		return err
	}

	if err := service.Send(message, nil); err != nil {
		return fmt.Errorf("%s: %w", service.GetID(), ErrSendFailed)
	}

	return nil
}

func (router *ServiceRouter) initService(rawURL string) (types.Service, error) {
	scheme, configURL, err := router.ExtractServiceName(rawURL)
	if err != nil {
		return nil, err
	}

	service, err := newService(scheme)
	if err != nil {
		return nil, err
	}

	if configURL.Scheme != scheme {
		router.log("Got custom URL:", configURL.String())

		customURLService, ok := service.(types.CustomURLService)
		if !ok {
			return nil, fmt.Errorf("%w: '%s' service", ErrCustomURLsNotSupported, scheme)
		}

		configURL, err = customURLService.GetConfigURLFromCustom(configURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", configURL.String(), ErrCustomURLConversion)
		}

		router.log("Converted service URL:", configURL.String())
	}

	err = service.Initialize(configURL, router.logger)
	if err != nil {
		return service, fmt.Errorf("%s: %w", scheme, ErrInitializeFailed)
	}

	return service, nil
}

// NewService returns a new uninitialized service instance.
func (*ServiceRouter) NewService(serviceScheme string) (types.Service, error) {
	return newService(serviceScheme)
}

// newService returns a new uninitialized service instance.
func newService(serviceScheme string) (types.Service, error) {
	serviceFactory, valid := serviceMap[strings.ToLower(serviceScheme)]
	if !valid {
		return nil, fmt.Errorf("%w: %q", ErrUnknownService, serviceScheme)
	}

	return serviceFactory(), nil
}

// ListServices returns the available services.
func (router *ServiceRouter) ListServices() []string {
	services := make([]string, len(serviceMap))

	i := 0

	for key := range serviceMap {
		services[i] = key
		i++
	}

	return services
}

// Locate returns the service implementation that corresponds to the given service URL.
func (router *ServiceRouter) Locate(rawURL string) (types.Service, error) {
	service, err := router.initService(rawURL)

	return service, err
}

func (router *ServiceRouter) log(v ...any) {
	if router.logger == nil {
		return
	}

	router.logger.Println(v...)
}
