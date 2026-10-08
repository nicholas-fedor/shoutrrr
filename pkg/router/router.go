package router

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ServiceRouter is responsible for routing a message to a specific notification service using the notification URL.
type ServiceRouter struct {
	logger   types.StdLogger
	services []types.Service
	queue    []string
	// Timeout, when positive, is the exact fixed timeout for every service.
	// Zero uses each service's own budget, with [DefaultTimeout] as the floor.
	Timeout     time.Duration
	httpClient  types.HTTPClient
	dialContext types.DialContextFunc
	//nolint:containedctx // Intentional: router derives per-service timeout contexts from this base.
	ctx context.Context
}

// DefaultTimeout is the send budget used when a service does not report a longer one.
const DefaultTimeout = types.DefaultSendTimeout

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
// services that support it (via SetHTTPClient). If opts.DialContext is non-nil,
// it will be injected into services that implement [types.DialContextSetter].
//
// Parameters:
//   - logger: the logger to use for service output.
//   - opts: the sender options, including timeout, HTTP client, and dial function.
//   - serviceURLs: the service URLs to initialize.
//
// Returns:
//   - *ServiceRouter: the initialized router.
//   - error: an error if any service fails to initialize.
func NewWithOptions(logger types.StdLogger, opts types.SenderOptions, serviceURLs ...string) (*ServiceRouter, error) {
	router := ServiceRouter{
		logger:      logger,
		services:    nil,
		queue:       nil,
		Timeout:     opts.Timeout,
		httpClient:  opts.HTTPClient,
		dialContext: opts.DialContext,
		ctx:         context.Background(),
	}

	for i, serviceURL := range serviceURLs {
		if err := router.AddService(serviceURL); err != nil {
			return nil, fmt.Errorf("error initializing router services: URL %d: %w", i, err)
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
		// Parse errors quote parts of the URL, such as an invalid port, which can
		// carry credentials, so the cause is not included.
		return "", &url.URL{}, ErrParseURLFailed
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
	if len(r.queue) == 0 {
		return
	}

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
		return &types.TargetError{URL: service.GetID(), Index: 0, Err: err}
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

	errs := make([]error, len(r.services))

	var waitGroup sync.WaitGroup

	for i, service := range r.services {
		serviceParams := cloneParams(params)

		waitGroup.Go(func() { errs[i] = r.sendToService(i, service, message, serviceParams) })
	}

	waitGroup.Wait()

	return errs
}

// SendAsync sends the specified message using the routers underlying services.
//
// Parameters:
//   - message: the message to send.
//   - params: the parameters to apply.
//
// Returns:
//   - chan error: a channel that receives one result per service in completion
//     order, then closes. Failures are *types.TargetError values whose Index
//     identifies the configured URL.
func (r *ServiceRouter) SendAsync(message string, params *types.Params) chan error {
	errs := make(chan error, len(r.services))

	var waitGroup sync.WaitGroup

	for i, service := range r.services {
		serviceParams := cloneParams(params)

		waitGroup.Go(func() { errs <- r.sendToService(i, service, message, serviceParams) })
	}

	go func() {
		waitGroup.Wait()
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

	errs := make([]error, len(r.services))

	var waitGroup sync.WaitGroup

	for i, service := range r.services {
		serviceParams := cloneParams(&params)

		waitGroup.Go(func() { errs[i] = r.sendItemsToService(i, service, items, serviceParams) })
	}

	waitGroup.Wait()

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

// baseContext returns the context that per-service send contexts derive from.
// A zero-value ServiceRouter has no context, so it falls back to the background
// context.
//
// Returns:
//   - context.Context: the base context for sends.
func (r *ServiceRouter) baseContext() context.Context {
	if r.ctx == nil {
		return context.Background()
	}

	return r.ctx
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
		// Custom URLs can carry credentials and headers, so log only the scheme.
		r.log("Got custom URL for service:", scheme)

		customURLService, ok := service.(types.CustomURLService)
		if !ok {
			return nil, fmt.Errorf("%w: '%s' service", ErrCustomURLsNotSupported, scheme)
		}

		convertedURL, err := customURLService.GetServiceURLFromCustom(serviceURL)
		if err != nil {
			return nil, fmt.Errorf("%w for '%s' service: %w", ErrCustomURLConversion, scheme, err)
		}

		if convertedURL == nil {
			return nil, fmt.Errorf("%w for '%s' service: no service URL returned", ErrCustomURLConversion, scheme)
		}

		serviceURL = convertedURL

		r.log("Converted custom URL for service:", scheme)
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
			// Redact the URLs in transport errors, which carry service credentials.
			setter.SetHTTPClient(redact.HTTPClient(r.httpClient))
		}
	}

	if r.dialContext != nil {
		if setter, ok := service.(types.DialContextSetter); ok {
			setter.SetDialContext(r.dialContext)
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

// sendBudget returns how long the router waits for one service.
//
// A positive Timeout is the exact fixed timeout for every service. Otherwise the
// wait is the greater of [DefaultTimeout] and the service's [types.ServiceTimeout].
//
// Parameters:
//   - service: The service being sent to.
//   - params: Send parameters passed through to ServiceTimeout.
//
// Returns:
//   - The budget for this send.
func (r *ServiceRouter) sendBudget(service types.Service, params *types.Params) time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}

	budget := DefaultTimeout

	serviceTimeout, ok := service.(types.ServiceTimeout)
	if !ok {
		return budget
	}

	if reported := serviceTimeout.ServiceTimeout(params); reported > budget {
		return reported
	}

	return budget
}

// sendItemsToService sends message items to a single service, respecting context and timeout.
//
// Parameters:
//   - index: the position of the service among the configured URLs.
//   - service: the service to send to.
//   - items: the message items to send.
//   - params: the parameters to apply, owned by this send.
//
// Returns:
//   - error: nil on success, otherwise a *types.TargetError for the service.
func (r *ServiceRouter) sendItemsToService(
	index int,
	service types.Service,
	items []types.MessageItem,
	params types.Params,
) error {
	result := make(chan error, 1)
	timeout := r.sendBudget(service, &params)

	sendCtx, cancel := context.WithTimeout(r.baseContext(), timeout)
	defer cancel()

	switch sender := service.(type) {
	case types.ContextAttachmentSender:
		go func() { result <- sender.SendItemsContext(sendCtx, items, params) }()
	case types.RichSender:
		go func() { result <- sender.SendItems(items, params) }()
	case types.ContextSender:
		go func() { result <- sender.SendContext(sendCtx, types.ItemsToPlain(items), &params) }()
	default:
		go func() { result <- service.Send(types.ItemsToPlain(items), &params) }()
	}

	return awaitResult(result, timeout, service.GetID(), index)
}

// sendToService sends a message to a single service, respecting context and timeout.
//
// Parameters:
//   - index: the position of the service among the configured URLs.
//   - service: the service to send to.
//   - message: the message to send.
//   - params: the parameters to apply, owned by this send.
//
// Returns:
//   - error: nil on success, otherwise a *types.TargetError for the service.
func (r *ServiceRouter) sendToService(
	index int,
	service types.Service,
	message string,
	params types.Params,
) error {
	result := make(chan error, 1)
	timeout := r.sendBudget(service, &params)

	if sender, ok := service.(types.ContextSender); ok {
		sendCtx, cancel := context.WithTimeout(r.baseContext(), timeout)
		defer cancel()

		go func() { result <- sender.SendContext(sendCtx, message, &params) }()
	} else {
		go func() { result <- service.Send(message, &params) }()
	}

	return awaitResult(result, timeout, service.GetID(), index)
}

// awaitResult waits for either the service result or a timeout, wrapping a
// failure in a *types.TargetError.
//
// Parameters:
//   - result: the channel carrying the service result.
//   - timeout: the operation timeout.
//   - serviceID: the identifier of the service for error wrapping.
//   - index: the position of the service among the configured URLs.
//
// Returns:
//   - error: nil on success, otherwise a *types.TargetError for the service.
func awaitResult(result <-chan error, timeout time.Duration, serviceID string, index int) error {
	select {
	case res := <-result:
		if res == nil {
			return nil
		}

		if errors.Is(res, context.DeadlineExceeded) {
			res = fmt.Errorf("%w: %v", ErrServiceTimeout, serviceID)
		}

		return &types.TargetError{URL: serviceID, Index: index, Err: res}
	case <-time.After(timeout):
		return &types.TargetError{
			URL:   serviceID,
			Index: index,
			Err:   fmt.Errorf("%w: %v", ErrServiceTimeout, serviceID),
		}
	}
}

// cloneParams returns a copy of params that a single service send can own.
//
// Parameters:
//   - params: the caller's params, which may be nil.
//
// Returns:
//   - types.Params: an independent copy, empty when params is nil.
func cloneParams(params *types.Params) types.Params {
	if params == nil || *params == nil {
		return types.Params{}
	}

	return maps.Clone(*params)
}
