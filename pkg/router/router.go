package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ServiceRouter is responsible for routing a message to a specific notification service using the notification URL.
type ServiceRouter struct {
	logger     types.StdLogger
	services   []types.Service
	queue      []string
	Timeout    time.Duration
	httpClient types.HTTPClient
	//nolint:containedctx // Intentional: router derives per-service timeout contexts from this base.
	ctx context.Context
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
)

// New creates a new service router using the specified logger and service URLs.
//
// Deprecated: Use NewWithOptions.
//
//go:fix inline
func New(logger types.StdLogger, serviceURLs ...string) (*ServiceRouter, error) {
	return NewWithOptions(logger, types.SenderOptions{}, serviceURLs...)
}

// NewWithOptions creates a new service router using the specified logger, options,
// and service URLs. If opts.HTTPClient is non-nil, it will be injected into
// services that support it (via SetHTTPClient or internal client replacement).
//
// Parameters:
//   - logger: the logger to use for service output.
//   - opts: the sender options, including timeout and HTTP client.
//   - serviceURLs: the service URLs to initialize.
//
// Returns:
//   - *ServiceRouter: the initialized router.
//   - error: an error if any service fails to initialize.
func NewWithOptions(logger types.StdLogger, opts types.SenderOptions, serviceURLs ...string) (*ServiceRouter, error) {
	router := ServiceRouter{
		logger:     logger,
		services:   nil,
		queue:      nil,
		Timeout:    DefaultTimeout,
		httpClient: opts.HTTPClient,
		ctx:        context.Background(),
	}

	if opts.Timeout > 0 {
		router.Timeout = opts.Timeout
	}

	for _, serviceURL := range serviceURLs {
		if err := router.AddService(serviceURL); err != nil {
			return nil, fmt.Errorf("error initializing router services: %w", err)
		}
	}

	return &router, nil
}

// AddService initializes the specified service from its URL, and adds it if no errors occur.
//
// Parameters:
//   - serviceURL: the service URL to initialize and add.
//
// Returns:
//   - error: an error if initialization fails.
func (r *ServiceRouter) AddService(serviceURL string) error {
	service, err := r.initService(serviceURL)
	if err == nil {
		r.services = append(r.services, service)
	}

	return err
}

// Enqueue adds the message to an internal queue and sends it when Flush is invoked.
//
// Parameters:
//   - message: the message to queue.
//   - v: optional format arguments for the message.
func (r *ServiceRouter) Enqueue(message string, v ...any) {
	if len(v) > 0 {
		message = fmt.Sprintf(message, v...)
	}

	r.queue = append(r.queue, message)
}

// ExtractServiceName extracts the service name from a service URL.
//
// Parameters:
//   - rawURL: the raw service URL.
//
// Returns:
//   - string: the extracted service scheme.
//   - *url.URL: the parsed URL.
//   - error: an error if parsing fails.
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

// Flush sends all messages that have been queued up as a combined message.
//
// Parameters:
//   - params: the parameters to apply to the combined message.
func (r *ServiceRouter) Flush(params *types.Params) {
	// Since this method is supposed to be deferred we just have to ignore errors
	_ = r.Send(strings.Join(r.queue, "\n"), params)
	r.queue = []string{}
}

// ListServices returns the available services.
//
// Returns:
//   - []string: the list of supported service schemas.
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
//
// Parameters:
//   - rawURL: the service URL to locate.
//
// Returns:
//   - types.Service: the located service implementation.
//   - error: an error if the service cannot be located or initialized.
func (r *ServiceRouter) Locate(rawURL string) (types.Service, error) {
	service, err := r.initService(rawURL)

	return service, err
}

// NewService returns a new uninitialized service instance.
//
// Parameters:
//   - serviceScheme: the service scheme to instantiate.
//
// Returns:
//   - types.Service: the new service instance.
//   - error: an error if the scheme is unknown.
func (*ServiceRouter) NewService(serviceScheme string) (types.Service, error) {
	return newService(serviceScheme)
}

// Route sends a message to a specific notification service using the notification URL.
//
// Parameters:
//   - rawURL: the service URL to send to.
//   - message: the message to send.
//
// Returns:
//   - error: an error if the send fails, wrapped in *types.TargetError.
func (r *ServiceRouter) Route(rawURL, message string) error {
	service, err := r.Locate(rawURL)
	if err != nil {
		return err
	}

	if err := service.Send(message, nil); err != nil {
		return &types.TargetError{URL: service.GetID(), Err: err}
	}

	return nil
}

// Send sends the specified message using the routers underlying services.
//
// Parameters:
//   - message: the message to send.
//   - params: the parameters to apply.
//
// Returns:
//   - []error: one error per service, in the same order as the configured URLs.
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
//
// Parameters:
//   - message: the message to send.
//   - params: the parameters to apply.
//
// Returns:
//   - chan error: a channel that will contain one error per service.
func (r *ServiceRouter) SendAsync(message string, params *types.Params) chan error {
	serviceCount := len(r.services)
	proxy := make(chan error, serviceCount)
	errs := make(chan error, serviceCount)

	if params == nil {
		params = &types.Params{}
	}

	for _, service := range r.services {
		go sendToService(service, proxy, r.Timeout, message, *params, r.ctx)
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
//
// Parameters:
//   - items: the message items to send.
//   - params: the parameters to apply.
//
// Returns:
//   - []error: one error per service, in the same order as the configured URLs.
func (r *ServiceRouter) SendItems(items []types.MessageItem, params types.Params) []error {
	if r == nil {
		return []error{ErrNoSenders}
	}

	serviceCount := len(r.services)
	proxy := make(chan error, serviceCount)
	errs := make([]error, serviceCount)

	for _, service := range r.services {
		go sendItemsToService(service, proxy, r.Timeout, items, params, r.ctx)
	}

	for i := range r.services {
		errs[i] = <-proxy
	}

	return errs
}

// SetLogger sets the logger that the services will use to write progress logs.
//
// Parameters:
//   - logger: the logger to set on all services.
func (r *ServiceRouter) SetLogger(logger types.StdLogger) {
	r.logger = logger
	for _, service := range r.services {
		service.SetLogger(logger)
	}
}

// initService initializes a service from the given URL.
//
// Parameters:
//   - rawURL: the raw service URL.
//
// Returns:
//   - types.Service: the initialized service.
//   - error: an error if initialization fails.
func (r *ServiceRouter) initService(rawURL string) (types.Service, error) {
	scheme, serviceURL, err := r.ExtractServiceName(rawURL)
	if err != nil {
		return nil, err
	}

	service, err := newService(scheme)
	if err != nil {
		return nil, err
	}

	if serviceURL.Scheme != scheme {
		r.log("Got custom URL:", serviceURL.String())

		customURLService, ok := service.(types.CustomURLService)
		if !ok {
			return nil, fmt.Errorf("%w: '%s' service", ErrCustomURLsNotSupported, scheme)
		}

		serviceURL, err = customURLService.GetServiceURLFromCustom(serviceURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", serviceURL.String(), ErrCustomURLConversion)
		}

		r.log("Converted service URL:", serviceURL.String())
	}

	err = service.Initialize(serviceURL, r.logger)
	if err != nil {
		return service, fmt.Errorf("%s: %w", scheme, err)
	}

	// Inject custom HTTP client if provided and the service supports it.
	if r.httpClient != nil {
		if client, ok := r.httpClient.(*http.Client); ok && client == nil {
			// skip typed-nil
		} else if setter, ok := service.(types.HTTPClientSetter); ok {
			setter.SetHTTPClient(r.httpClient)
		}
	}

	return service, nil
}

// log writes a log message if a logger is configured.
//
// Parameters:
//   - v: the values to log.
func (r *ServiceRouter) log(v ...any) {
	if r.logger == nil {
		return
	}

	r.logger.Println(v...)
}

// newService returns a new uninitialized service instance.
func newService(serviceScheme string) (types.Service, error) {
	serviceFactory, valid := serviceMap[strings.ToLower(serviceScheme)]
	if !valid {
		return nil, fmt.Errorf("%w: %q", ErrUnknownService, serviceScheme)
	}

	return serviceFactory(), nil
}

// awaitResult waits for either the service result or a timeout, wrapping the
// result in a TargetError if needed.
//
// Parameters:
//   - results: the channel to report the final error to.
//   - result: the channel carrying the service result.
//   - timeout: the operation timeout.
//   - serviceID: the identifier of the service for error wrapping.
func awaitResult(results chan error, result <-chan error, timeout time.Duration, serviceID string) {
	select {
	case res := <-result:
		if res != nil {
			if errors.Is(res, context.DeadlineExceeded) {
				res = &types.TargetError{URL: serviceID, Err: fmt.Errorf("%w: %v", ErrServiceTimeout, serviceID)}
			} else {
				res = &types.TargetError{URL: serviceID, Err: res}
			}
		}

		results <- res
	case <-time.After(timeout):
		results <- &types.TargetError{URL: serviceID, Err: fmt.Errorf("%w: %v", ErrServiceTimeout, serviceID)}
	}
}

// sendToService sends a message to a single service, respecting context and timeout.
//
// Parameters:
//   - service: the service to send to.
//   - results: the channel to report the result error to.
//   - timeout: the operation timeout.
//   - message: the message to send.
//   - params: the parameters to apply.
//   - ctx: the base context for the operation.
func sendToService(
	service types.Service,
	results chan error,
	timeout time.Duration,
	message string,
	params types.Params,
	ctx context.Context,
) {
	result := make(chan error, 1)

	serviceID := service.GetID()

	if sender, ok := service.(types.ContextSender); ok {
		sendCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		go func() { result <- sender.SendContext(sendCtx, message, &params) }()
	} else {
		go func() { result <- service.Send(message, &params) }()
	}

	awaitResult(results, result, timeout, serviceID)
}

// sendItemsToService sends message items to a single service, respecting context and timeout.
//
// Parameters:
//   - service: the service to send to.
//   - results: the channel to report the result error to.
//   - timeout: the operation timeout.
//   - items: the message items to send.
//   - params: the parameters to apply.
//   - ctx: the base context for the operation.
func sendItemsToService(
	service types.Service,
	results chan error,
	timeout time.Duration,
	items []types.MessageItem,
	params types.Params,
	ctx context.Context,
) {
	result := make(chan error, 1)

	serviceID := service.GetID()

	switch sender := service.(type) {
	case types.ContextAttachmentSender:
		sendCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		go func() { result <- sender.SendItemsContext(sendCtx, items, params) }()
	case types.RichSender:
		go func() { result <- sender.SendItems(items, params) }()
	case types.ContextSender:
		sendCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		go func() { result <- sender.SendContext(sendCtx, types.ItemsToPlain(items), &params) }()
	default:
		go func() { result <- service.Send(types.ItemsToPlain(items), &params) }()
	}

	awaitResult(results, result, timeout, serviceID)
}
