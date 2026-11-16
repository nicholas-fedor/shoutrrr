package pagerduty

// EventPayload represents the payload being sent to the PagerDuty Events API v2.
//
// See: https://developer.pagerduty.com/docs/events-api-v2-overview
type EventPayload struct {
	// Details is not sent.
	//
	// Deprecated: Use [Payload.CustomDetails]. The Events API v2 has no
	// top-level details field.
	Details any `json:"-"`
	// Payload contains the core incident information including summary,
	// severity, and source. It is left out when it is empty, as for an
	// acknowledge or resolve event.
	Payload Payload `json:"payload,omitzero"`
	// RoutingKey is the integration key used to route the event to the correct PagerDuty service.
	RoutingKey string `json:"routing_key"`
	// EventAction specifies the type of event (e.g., "trigger", "acknowledge", "resolve").
	EventAction string `json:"event_action"`
	// DedupKey identifies the alert. PagerDuty generates one for a trigger event
	// without it, and an acknowledge or resolve event needs it.
	DedupKey string `json:"dedup_key,omitempty"`
	// Client identifies the monitoring client or integration sending the event.
	Client string `json:"client,omitempty"`
	// ClientURL is a link to the monitoring client or integration dashboard.
	ClientURL string `json:"client_url,omitempty"`
	// Contexts is not sent.
	//
	// Deprecated: Use [EventPayload.Links] and [EventPayload.Images]. The Events
	// API v2 has no contexts field.
	Contexts []PagerDutyContext `json:"-"`
	// Links are text links attached to the incident.
	Links []Link `json:"links,omitempty"`
	// Images are images attached to the incident.
	Images []Image `json:"images,omitempty"`
}

// Payload contains the core information about the incident or event being reported.
// It includes a summary description, severity level, and the source of the event.
type Payload struct {
	// CustomDetails holds additional details about the event and the affected system.
	CustomDetails any `json:"custom_details,omitzero"`
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

// Link is a text link attached to an incident.
type Link struct {
	// Href is the URL of the link.
	Href string `json:"href"`
	// Text describes the link and can be shown as its text.
	Text string `json:"text,omitempty"`
}

// Image is an image attached to an incident.
type Image struct {
	// Src is the URL of the image, which PagerDuty requires to use HTTPS.
	Src string `json:"src"`
	// Href makes the image a link to this URL.
	Href string `json:"href,omitempty"`
	// Alt is the alternative text for the image.
	Alt string `json:"alt,omitempty"`
}

// messageDetails holds the custom details of a trigger event whose details
// prop is empty and whose message is not the summary.
type messageDetails struct {
	// Message is the notification message.
	Message string `json:"message"`
}
