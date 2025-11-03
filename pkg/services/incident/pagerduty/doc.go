// Package pagerduty provides a notification service for PagerDuty incident management.
//
// The PagerDuty service enables sending alerts, acknowledgments, and resolutions to PagerDuty
// using the Events API v2. It supports creating incidents, acknowledging existing incidents,
// and resolving incidents with configurable severity levels and custom parameters.
//
// # URL Format
//
// The service URL follows the format:
//
//	pagerduty://[host[:port]]/integration-key[?query]
//
// Where:
//   - host: optional PagerDuty API host (default: events.pagerduty.com)
//   - port: optional port number (default: 443)
//   - integration-key: the PagerDuty Events API v2 integration key
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - severity: alert severity (critical, error, warning, info; default: error)
//   - source: unique location of affected system (default: default)
//   - action: event type (trigger, acknowledge, resolve; default: trigger)
//
// # Event Actions
//
// ## Trigger (Default)
//
// Creates a new incident in PagerDuty:
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889"
//	err := shoutrrr.Send(url, "Server is down")
//
// ## Acknowledge
//
// Acknowledges an existing incident:
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?action=acknowledge"
//	err := shoutrrr.Send(url, "Investigating server issue")
//
// ## Resolve
//
// Resolves an existing incident:
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?action=resolve"
//	err := shoutrrr.Send(url, "Server issue resolved")
//
// # Examples
//
// ## Basic incident creation
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889"
//	err := shoutrrr.Send(url, "Database connection failed")
//
// ## Critical severity alert
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?severity=critical"
//	err := shoutrrr.Send(url, "Production system unavailable")
//
// ## Custom source identifier
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?source=web-server-01"
//	err := shoutrrr.Send(url, "High CPU usage detected")
//
// ## Using parameters in code
//
//	service.Send("Alert message", &types.Params{
//		"severity": "critical",
//		"source":   "monitoring-system",
//		"action":   "trigger",
//	})
//
// ## Command line usage
//
//	shoutrrr send -u 'pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?severity=critical&source=app' -m 'Application crashed'
//
// # Common Use Cases
//
// ## Monitoring System Integration
//
// Send alerts from monitoring tools like Prometheus, Nagios, or Zabbix:
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?source=prometheus"
//	err := shoutrrr.Send(url, "CPU usage above 90%", map[string]string{"severity": "warning"})
//
// ## Application Error Notifications
//
// Notify PagerDuty of application errors or exceptions:
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?severity=error&source=my-app"
//	err := shoutrrr.Send(url, fmt.Sprintf("Exception in %s: %v", functionName, err))
//
// ## Infrastructure Alerts
//
// Alert on infrastructure issues like disk space, memory, or network problems:
//
//	url := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?severity=critical&source=server-01"
//	err := shoutrrr.Send(url, "Disk space critically low: 95% used")
//
// ## Automated Incident Response
//
// Use acknowledge and resolve actions in automated workflows:
//
//	// Acknowledge incident
//	ackURL := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?action=acknowledge"
//	shoutrrr.Send(ackURL, "Auto-acknowledging for investigation")
//
//	// Resolve incident
//	resolveURL := "pagerduty:///eb243592-faa2-4ba2-a551q-1afdf565c889?action=resolve"
//	shoutrrr.Send(resolveURL, "Issue automatically resolved")
//
// # Error Handling
//
// The service returns errors for network failures, invalid integration keys,
// and PagerDuty API errors. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//		log.Printf("Failed to send PagerDuty alert: %v", err)
//	}
//
// Common error scenarios:
//   - Invalid integration key: HTTP 400 Bad Request
//   - Network connectivity issues: connection timeouts
//   - API rate limiting: HTTP 429 Too Many Requests
//   - Server errors: HTTP 5xx status codes
//
// # Security Considerations
//
// - Store integration keys securely, never in version control
// - Use HTTPS for all PagerDuty API communications
// - Validate integration keys before deployment
// - Monitor API usage to avoid rate limiting
// - Consider IP allowlisting for production environments
// - Use appropriate severity levels to avoid alert fatigue
//
// # API Reference
//
// This service uses the PagerDuty Events API v2. For detailed API documentation,
// see: https://developer.pagerduty.com/docs/events-api-v2-overview
//
// The payload structure includes:
//   - routing_key: integration key for event routing
//   - event_action: trigger, acknowledge, or resolve
//   - payload: contains summary, severity, and source information
package pagerduty
