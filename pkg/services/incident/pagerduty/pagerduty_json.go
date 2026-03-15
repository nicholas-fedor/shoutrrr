package pagerduty

// EventPayload represents the payload being sent to the PagerDuty Events API v2.
//
// See: https://developer.pagerduty.com/docs/events-api-v2-overview
type EventPayload struct {
	// Payload contains the core incident information including summary, severity, and source.
	Payload Payload `json:"payload"`
	// RoutingKey is the integration key used to route the event to the correct PagerDuty service.
	RoutingKey string `json:"routing_key"`
	// EventAction specifies the type of event (e.g., "trigger", "acknowledge", "resolve").
	EventAction string `json:"event_action"`
	// DedupKey is a unique key used for incident deduplication. If not provided, PagerDuty will auto-generate one.
	DedupKey string `json:"dedup_key,omitempty"`
	// Details contains additional custom key-value pairs for the incident.
	Details any `json:"details,omitempty"`
	// Contexts provides additional context such as links or images related to the incident.
	Contexts []PagerDutyContext `json:"contexts,omitempty"`
	// Client identifies the monitoring client or integration sending the event.
	Client string `json:"client,omitempty"`
	// ClientURL is a link to the monitoring client or integration dashboard.
	ClientURL string `json:"client_url,omitempty"`
}

// Payload contains the core information about the incident or event being reported.
// It includes a summary description, severity level, and the source of the event.
type Payload struct {
	// Summary is a brief description of the incident or event.
	Summary string `json:"summary"`
	// Severity indicates the urgency level of the event (e.g., "info", "warning", "error", "critical").
	Severity string `json:"severity"`
	// Source identifies the origin of the event, such as a hostname, application name, or service.
	Source string `json:"source"`
}

// PagerDutyContext provides additional context about the incident, such as links to related resources.
type PagerDutyContext struct {
	// Type specifies the type of context (e.g., "link", "image").
	Type string `json:"type"`
	// Href is the URL for link-type contexts.
	Href string `json:"href,omitempty"`
	// Src is the source URL for image-type contexts.
	Src string `json:"src,omitempty"`
	// Text is the display text for the context.
	Text string `json:"text,omitempty"`
}
