// Package opsgenie provides a notification service for sending alerts to
// OpsGenie or to Jira Service Management's alert API.
//
// OpsGenie is an alert management platform that helps teams respond to incidents
// quickly and efficiently. This package integrates with OpsGenie's REST API to
// create alerts from various notification sources. Atlassian ends support for
// OpsGenie on April 5, 2027. The same alerts can go to Jira Service Management,
// which takes the same request.
//
// # Overview
//
// The package implements the shoutrrr Service interface, allowing it to be used
// as a notification destination for incident management workflows. It supports
// sending alerts with full customization including priority, responders, tags,
// actions, and detailed metadata.
//
// # URL Scheme
//
// The service uses the "opsgenie" URL scheme for configuration:
//
//	opsgenie://[host[:port]]/<api-key>[?<additional-parameters>]
//
// # Configuration Examples
//
// Basic usage with the default US API host:
//
//	opsgenie:///API_KEY
//
// EU instance configuration:
//
//	opsgenie://api.eu.opsgenie.com/API_KEY
//
// Jira Service Management's alert API:
//
//	opsgenie://api.atlassian.com/API_KEY
//
// Full configuration with alert options:
//
//	opsgenie://api.opsgenie.com:443/API_KEY?priority=P1&tags=production,critical&actions=acknowledge,resolve
//
// # Key Types
//
// The package provides several key types for configuring and sending alerts:
//
// Service (opsgenie.go)
//
//	The main service type that implements the shoutrrr Service interface.
//	Use Initialize() to configure the service with a URL, and Send() to deliver
//	notifications to OpsGenie.
//
//	Config (opsgenie_config.go)
//
//	Contains all configuration options for the OpsGenie service:
//	  - APIKey: Your OpsGenie API key (required)
//	  - Host: API endpoint host (defaults to api.opsgenie.com)
//	  - Port: API endpoint port (defaults to 443)
//	  - Alert properties: Alias, Description, Responders, VisibleTo, Actions,
//	    Tags, Details, Entity, Source, Priority, User, Note, Title
//
//	Entity (opsgenie_entity.go)
//
//	Represents OpsGenie entities such as teams, users, escalations, or schedules.
//	Entities are specified using the format "type:identifier" where type can be
//	"team", "user", "escalation", or "schedule", and identifier can be either
//	an ID or name/username.
//
//	AlertPayload (opsgenie_json.go)
//
//	Represents the JSON payload sent to the OpsGenie Create Alert API.
//	This struct maps directly to the OpsGenie API request body.
//
// # Message Length Limits
//
// OpsGenie has a maximum message length of 130 characters. When the message
// exceeds this limit, the service automatically truncates the message to use
// as the alert title, while using the full message as the description.
//
// OpsGenie requires an alert message, so a send without a title or a message
// fails with [ErrMessageMissing] before the request.
//
// # API Endpoints
//
// The service sends alerts to the Create Alert API of the host. Host
// api.atlassian.com gets Jira Service Management's path, and every other host
// gets OpsGenie's:
//
//	US Instance: https://api.opsgenie.com/v2/alerts
//	EU Instance: https://api.eu.opsgenie.com/v2/alerts
//	Jira Service Management: https://api.atlassian.com/jsm/ops/integration/v2/alerts
//
// # Authentication
//
// Authentication is performed using the OpsGenie API key (GenieKey) passed in
// the Authorization header:
//
//	Authorization: GenieKey <api-key>
//
// # See Also
//
// For more information about the OpsGenie Alert API:
// https://docs.opsgenie.com/docs/alert-api
//
// For more information about OpsGenie priorities:
// https://docs.opsgenie.com/docs/alert-priority
//
// For more information about OpsGenie entity types:
// https://docs.opsgenie.com/docs/responders
//
// For more information about Jira Service Management's alert API:
// https://developer.atlassian.com/cloud/jira/service-desk-ops/rest/v1/api-group-integration-events/
package opsgenie
