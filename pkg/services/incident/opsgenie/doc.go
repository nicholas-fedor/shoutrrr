// Package opsgenie provides a notification service for sending alerts to OpsGenie.
//
// OpsGenie is an alert management platform that helps teams respond to incidents
// quickly and efficiently. This package integrates with OpsGenie's REST API to
// create alerts from various notification sources.
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
//	opsgenie://<api-key>?host=<host>&port=<port>&<additional-parameters>
//
// # Configuration Examples
//
// Basic usage with default US API endpoint:
//
//	opsgenie://your-api-key-here
//
// EU instance configuration:
//
//	opsgenie://your-api-key-here?host=api.eu.opsgenie.com
//
// Full configuration with all options:
//
//	opsgenie://your-api-key-here?host=api.opsgenie.com&port=443&priority=P1&tags=production,critical&actions=acknowledge,resolve
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
// # API Endpoints
//
// The service sends alerts to the OpsGenie Create Alert API:
//
//	US Instance: https://api.opsgenie.com/v2/alerts
//	EU Instance: https://api.eu.opsgenie.com/v2/alerts
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
package opsgenie
