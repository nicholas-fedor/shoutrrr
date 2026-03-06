package router

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ServiceRouter is responsible for routing a message to a specific notification service using the notification URL.
type ServiceRouter struct {
	logger   types.StdLogger
	services []types.Service
	queue    []string
	Timeout  time.Duration
}

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

// AddService initializes the specified service from its URL, and adds it if no errors occur.
func (r *ServiceRouter) AddService(serviceURL string) error {
	service, err := r.initService(serviceURL)
	if err == nil {
		r.services = append(r.services, service)
	}

	return err
}

// Enqueue adds the message to an internal queue and sends it when Flush is invoked.
func (r *ServiceRouter) Enqueue(message string, v ...any) {
	if len(v) > 0 {
		message = fmt.Sprintf(message, v...)
	}

	r.queue = append(r.queue, message)
}

// ExtractServiceName from a notification URL.
func (r *ServiceRouter) ExtractServiceName(rawURL string) (string, *url.URL, error) {
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

// Flush sends all messages that have been queued up as a combined message. This method should be deferred!
func (r *ServiceRouter) Flush(params *types.Params) {
	// Since this method is supposed to be deferred we just have to ignore errors
	_ = r.Send(strings.Join(r.queue, "\n"), params)
	r.queue = []string{}
}

// ListServices returns the available services.
func (r *ServiceRouter) ListServices() []string {
	services := make([]string, len(serviceMap))

	i := 0

	for key := range serviceMap {
		services[i] = key
		i++
	}

	return services
}

// Locate returns the service implementation that corresponds to the given service URL.
func (r *ServiceRouter) Locate(rawURL string) (types.Service, error) {
	service, err := r.initService(rawURL)

	return service, err
}

// NewService returns a new uninitialized service instance.
func (*ServiceRouter) NewService(serviceScheme string) (types.Service, error) {
	return newService(serviceScheme)
}

// Route a message to a specific notification service using the notification URL.
func (r *ServiceRouter) Route(rawURL, message string) error {
	service, err := r.Locate(rawURL)
	if err != nil {
		return err
	}

	if err := service.Send(message, nil); err != nil {
		return fmt.Errorf("%s: %w", service.GetID(), ErrSendFailed)
	}

	return nil
}

// Send sends the specified message using the routers underlying services.
func (r *ServiceRouter) Send(message string, params *types.Params) []error {
	if r == nil {
		return []error{ErrNoSenders}
	}

	serviceCount := len(r.services)
	errs := make([]error, serviceCount)
	results := r.SendAsync(message, params)

	for i := range r.services {
		errs[i] = <-results
	}

	return errs
}

// SendAsync sends the specified message using the routers underlying services.
func (r *ServiceRouter) SendAsync(message string, params *types.Params) chan error {
	serviceCount := len(r.services)
	proxy := make(chan error, serviceCount)
	errs := make(chan error, serviceCount)

	if params == nil {
		params = &types.Params{}
	}

	for _, service := range r.services {
		go sendToService(service, proxy, r.Timeout, message, *params)
	}

	go func() {
		for range serviceCount {
			errs <- <-proxy
		}

		close(errs)
	}()

	return errs
}

// SendItems sends the specified message items using the routers underlying services.
func (r *ServiceRouter) SendItems(items []types.MessageItem, params types.Params) []error {
	if r == nil {
		return []error{ErrNoSenders}
	}

	// Fallback using old API for now
	message := strings.Builder{}
	for _, item := range items {
		message.WriteString(item.Text)
	}

	serviceCount := len(r.services)
	errs := make([]error, serviceCount)
	results := r.SendAsync(message.String(), &params)

	for i := range r.services {
		errs[i] = <-results
	}

	return errs
}

// SetLogger sets the logger that the services will use to write progress logs.
func (r *ServiceRouter) SetLogger(logger types.StdLogger) {
	r.logger = logger
	for _, service := range r.services {
		service.SetLogger(logger)
	}
}

func (r *ServiceRouter) initService(rawURL string) (types.Service, error) {
	scheme, configURL, err := r.ExtractServiceName(rawURL)
	if err != nil {
		return nil, err
	}

	service, err := newService(scheme)
	if err != nil {
		return nil, err
	}

	if configURL.Scheme != scheme {
		r.log("Got custom URL:", configURL.String())

		customURLService, ok := service.(types.CustomURLService)
		if !ok {
			return nil, fmt.Errorf("%w: '%s' service", ErrCustomURLsNotSupported, scheme)
		}

		configURL, err = customURLService.GetConfigURLFromCustom(configURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", configURL.String(), ErrCustomURLConversion)
		}

		r.log("Converted service URL:", configURL.String())
	}

	err = service.Initialize(configURL, r.logger)
	if err != nil {
		return service, fmt.Errorf("%s: %w", scheme, ErrInitializeFailed)
	}

	return service, nil
}

func (r *ServiceRouter) log(v ...any) {
	if r.logger == nil {
		return
	}

	r.logger.Println(v...)
}

// New creates a new service router using the specified logger and service URLs.
func New(logger types.StdLogger, serviceURLs ...string) (*ServiceRouter, error) {
	router := ServiceRouter{
		logger:   logger,
		services: nil,
		queue:    nil,
		Timeout:  DefaultTimeout,
	}

	for _, serviceURL := range serviceURLs {
		if err := router.AddService(serviceURL); err != nil {
			return nil, fmt.Errorf("error initializing router services: %w", err)
		}
	}

	return &router, nil
}

// newService returns a new uninitialized service instance.
func newService(serviceScheme string) (types.Service, error) {
	serviceFactory, valid := serviceMap[strings.ToLower(serviceScheme)]
	if !valid {
		return nil, fmt.Errorf("%w: %q", ErrUnknownService, serviceScheme)
	}

	return serviceFactory(), nil
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
