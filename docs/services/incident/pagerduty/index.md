# PagerDuty

## Overview

PagerDuty is a digital operations management platform that helps teams respond to incidents and outages. The PagerDuty service integrates with PagerDuty's Events API v2 to create, acknowledge, and resolve incidents programmatically. This service enables automated incident management workflows, allowing you to trigger alerts when issues are detected, acknowledge them when investigation begins, and resolve them when problems are fixed.

The service supports rich incident context including custom details, links to related resources, images, and client information to provide comprehensive incident information to on-call responders.

## URL Format

--8<-- "docs/services/incident/pagerduty/config.md"

### URL Components

The PagerDuty service URL follows this structure:

!!! info ""
    pagerduty://[__`HOST`__[:__`PORT`__]]/__`INTEGRATION_KEY`__[?__`QUERY_PARAMETERS`__]

__Components:__

- __Scheme__: `pagerduty://` - Identifies the service type
- `HOST` (optional): PagerDuty API hostname (default: `events.pagerduty.com`)
- `PORT` (optional): API port number (default: `443` for HTTPS)
- `INTEGRATION_KEY`: 32-character hexadecimal string that routes events to your PagerDuty service
- `QUERY_PARAMETERS` (optional): Additional configuration parameters

### Examples

!!! example "Default configuration"
    ```url
    pagerduty:///INTEGRATION_KEY
    ```
    Uses default host (`events.pagerduty.com`) and port (`443`)

!!! example "Custom host"
    ```url
    pagerduty://events.pagerduty.com/INTEGRATION_KEY
    ```
    Explicitly specifies the PagerDuty Events API host

!!! example "With parameters"
    ```url
    pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=critical&source=production-db
    ```
    Includes severity and source parameters

## Setup and Service Integration

### Prerequisites

Before using the PagerDuty service, you need:

1. A PagerDuty account
2. A service configured in PagerDuty
3. An Events API v2 integration key

### Creating a Service Integration

Follow these steps to set up PagerDuty integration:

1. __Log in to PagerDuty__
   - Access your PagerDuty account at [pagerduty.com](https://pagerduty.com)

2. __Navigate to Services__
   - Go to __Services__ → __Service Directory__ in the main navigation

3. __Create or Select a Service__
   - Either create a new service or select an existing one
   - If creating new: Click __+ New Service__, give it a name, and select an escalation policy

4. __Add Events API v2 Integration__
   - In the service details, click __Integrations__ tab
   - Click __+ Add another integration__
   - Search for and select __Events API v2__
   - Give the integration a name (e.g., "Shoutrrr Integration")
   - Click __Create Integration__

5. __Copy Integration Key__
   - The integration key will be displayed
   - Copy this 32-character key for use in your Shoutrrr URL

!!! tip "Integration Key Security"
    Keep your integration key secure. It acts as an authentication token for sending events to PagerDuty.

!!! note "Only Integration Keys Supported"
    This service only accepts PagerDuty service integration keys (32 hexadecimal characters matching `/^[a-fA-F0-9]{32}$/`). API keys are not supported and will result in an invalid integration key error.

## Parameters

The PagerDuty service supports the following parameters to customize incident creation and management:

### Core Parameters

| Parameter | Type | Description | Required | Default | Valid Values |
| ----------- | ------ | ------------- | ---------- | --------- | -------------- |
| `severity` | string | The perceived severity of the incident | No | `error` | `info`, `warning`, `error`, `critical` |
| `source` | string | The unique location of the affected system | No | `default` | Any string identifier |
| `action` | string | The type of event to send | No | `trigger` | `trigger`, `acknowledge`, `resolve` |
| `dedup_key` | string | Identifies the alert the event opens or changes | For `acknowledge` and `resolve` | - | At most 255 characters |
| `title` | string | The summary of a trigger event | No | - | Any string |

A `severity` or `action` outside its valid values is rejected when the URL is read, and a send whose params set one fails before any request.

### Additional Parameters

| Parameter | Type | Description | Required | Default | Valid Values |
| ----------- | ------ | ------------- | ---------- | --------- | -------------- |
| `details` | object | Additional custom key-value pairs for the incident | No | - | JSON object or URL-encoded string |
| `contexts` | array | Additional context such as links or images | No | - | JSON array of context objects (only 'link' and 'image' types supported) |
| `client` | string | Name of the monitoring client/integration | No | - | Any string |
| `client_url` | string | URL of the monitoring client dashboard | No | - | Valid URL |

### Parameter Details

#### Severity Levels

- __`info`__: Informational events that don't require immediate action
- __`warning`__: Potential issues that should be monitored
- __`error`__: Actual problems affecting system operation
- __`critical`__: Urgent issues requiring immediate response

A sender can set a message level with the `level` param, as `Params.SetLevel` does.
When neither the URL nor the send params set `severity`, the level picks it:

| Level   | Severity  |
|---------|-----------|
| Debug   | `info`    |
| Info    | `info`    |
| Warning | `warning` |
| Error   | `error`   |

`critical` is left for events that configure it.
A `severity` in the URL or the send params takes precedence over the level, even when it is set to the default.

#### Event Action Items

- __`trigger`__: Opens an alert, or adds to the open alert with the same `dedup_key`
- __`acknowledge`__: Acknowledges the alert that `dedup_key` names
- __`resolve`__: Resolves the alert that `dedup_key` names

#### Title and Message

A trigger event's summary is the title when one is set, and the message otherwise.
PagerDuty uses the summary as the alert's title and limits it to 1024 characters.

| Title | Summary | Custom details `message` |
| ------- | --------- | -------------------------- |
| Set | The title | The message |
| Not set, message up to 1024 characters | The message | Not sent |
| Not set, longer message | The first 1024 characters of the message | The whole message |

A `message` key in the `details` parameter keeps its own value.

An acknowledge or resolve event carries only the routing key, the action, and the `dedup_key`, as in PagerDuty's examples.
Shoutrrr does not send its message, title, or other event fields.

#### Details Parameter

The `details` parameter accepts a JSON object with custom key-value pairs.
Shoutrrr sends it as the event's `payload.custom_details`:

```json
{
  "component": "database",
  "version": "2.1.0",
  "error_code": "ECONNREFUSED",
  "affected_users": 150
}
```

#### Contexts Parameter

The `contexts` parameter accepts a JSON array of context objects. Each context must have a `type` field. Only `link` and `image` types are supported.
Shoutrrr sends link contexts in the event's `links` array and image contexts in its `images` array.
The parameter also takes comma-separated `link:URL` and `image:URL` entries, and any other type is rejected in either form.
A context's `text` becomes the link text or the image's alternative text.
PagerDuty requires image sources to use HTTPS.

__Link contexts:__

```json
{
  "type": "link",
  "href": "https://example.com/logs",
  "text": "View Application Logs"
}
```

__Image contexts:__

```json
{
  "type": "image",
  "src": "https://example.com/error-screenshot.png",
  "href": "https://example.com/dashboard",
  "text": "Error Screenshot"
}
```

## Usage Examples

### Basic Incident Triggering

!!! example "Simple alert"
    ```url
    pagerduty:///INTEGRATION_KEY
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///INTEGRATION_KEY' -m 'Database connection lost'
    ```

!!! example "Critical incident with source"
    ```url
    pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=critical&source=prod-db-01
    ```
    ```bash
    shoutrrr send -u 'pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=critical&source=prod-db-01' -m 'Production database is unresponsive'
    ```

### Incident Management Workflow

The trigger event names its alert with a `dedup_key`, so the later events can change it.
Replace `DEDUP_KEY` with a key of your choice, such as `db-connection-prod`.

!!! example "Trigger incident"
    ```url
    pagerduty:///INTEGRATION_KEY?dedup_key=DEDUP_KEY
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?dedup_key=DEDUP_KEY' -m 'Database connection lost'
    ```

!!! example "Acknowledge incident"
    ```url
    pagerduty:///INTEGRATION_KEY?action=acknowledge&dedup_key=DEDUP_KEY
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?action=acknowledge&dedup_key=DEDUP_KEY' -m 'Acknowledged'
    ```

!!! example "Resolve incident"
    ```url
    pagerduty:///INTEGRATION_KEY?action=resolve&dedup_key=DEDUP_KEY
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?action=resolve&dedup_key=DEDUP_KEY' -m 'Resolved'
    ```

### Advanced Examples

!!! example "With custom details"
    ```url
    pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=warning&source=monitoring&details={"component":"api","version":"1.2.3","response_time":"5.2s"}
    ```
    ```bash
    shoutrrr send -u 'pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=warning&source=monitoring&details={"component":"api","version":"1.2.3","response_time":"5.2s"}' -m 'API response time degraded'
    ```

!!! example "With contexts and client info"
    ```url
    pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=error&source=ci-pipeline&contexts=[{"type":"link","href":"https://github.com/org/repo/actions/runs/123","text":"GitHub Actions Run"},{"type":"image","src":"https://example.com/failure-screenshot.png","text":"Build Failure"}]&client=jenkins&client_url=https://ci.example.com/job/123
    ```
    ```bash
    shoutrrr send -u 'pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=error&source=ci-pipeline&contexts=[{"type":"link","href":"https://github.com/org/repo/actions/runs/123","text":"GitHub Actions Run"},{"type":"image","src":"https://example.com/failure-screenshot.png","text":"Build Failure"}]&client=jenkins&client_url=https://ci.example.com/job/123' -m 'CI/CD pipeline failed'
    ```

## Event Actions

The PagerDuty service supports three primary event actions that correspond to the incident lifecycle:

### Trigger (`action=trigger`)

Creates a new incident or updates an existing one with new information. This is the default action.

__Use cases:__

- System failures or outages
- Performance degradation
- Security alerts
- Application errors

__Behavior:__

- Opens an alert, or adds a trigger to the open alert with the same `dedup_key`
- Includes all provided context, details, and parameters

### Acknowledge (`action=acknowledge`)

Acknowledges an existing incident, stopping escalation and indicating that someone is actively working on it.

__Use cases:__

- Engineer has started investigation
- Issue has been triaged
- Response team is assembled

__Behavior:__

- Stops incident escalation
- Needs the `dedup_key` of the alert

### Resolve (`action=resolve`)

Resolves an incident, marking it as completed and closing the alert cycle.

__Use cases:__

- Issue has been fixed
- Service has been restored
- Incident is no longer relevant

__Behavior:__

- Closes the incident
- Needs the `dedup_key` of the alert

!!! note "Alert Deduplication"
    PagerDuty applies an event to the open alert with the same `dedup_key` and routing key.
    A trigger event without a `dedup_key` gets a generated one and always opens a new alert.
    An acknowledge or resolve event for an alert that is not open is dropped.
    See PagerDuty's [Alert De-Duplication](https://docs.pagerduty.com/developer/send-alert-event#alert-de-duplication) section.

## Advanced Features

### Custom Details

The `details` parameter allows you to attach structured data to incidents:

```json
{
  "environment": "production",
  "component": "web-server",
  "version": "2.1.0",
  "error_code": "500",
  "affected_services": ["api", "database"],
  "metrics": {
    "cpu_usage": 95.2,
    "memory_usage": 87.1
  }
}
```

These details appear in the PagerDuty incident details and can be used for filtering, reporting, and automation.

### Contexts

Contexts provide additional visual and navigational context to responders:

__Link Contexts:__

- Direct responders to relevant resources
- Include runbooks, documentation, or monitoring dashboards
- Support both internal and external links

__Image Contexts:__

- Attach screenshots or graphs
- Show error visualizations or system diagrams
- Include clickable links to full-size images

### Client Information

The `client` and `client_url` parameters identify the monitoring system:

- __`client`__: Name of your monitoring tool (e.g., "Prometheus", "DataDog", "Custom Monitor")
- __`client_url`__: Direct link to the monitoring dashboard or alert details

This helps responders understand where the alert originated and provides quick access to additional monitoring data.

### Incident Grouping

PagerDuty supports incident grouping based on custom details. Use consistent detail keys to group related incidents:

```json
{
  "service": "payment-api",
  "region": "us-west-2",
  "cluster": "production"
}
```

## Rate Limiting

PagerDuty limits how many events a service accepts at a time and adjusts the limit to the incoming traffic.
An event over the limit gets HTTP 429 (Too Many Requests), and PagerDuty advises retrying it after a backoff of a few minutes.
Shoutrrr does not retry it.

### Handling Rate Limits

When rate limits are exceeded:

1. __Implement backoff__: Use exponential backoff (start with 1 minute, double each retry)
2. __Queue messages__: Buffer notifications during high-volume periods
3. __Use multiple integrations__: Distribute load across multiple integration keys if needed
4. __Monitor usage__: Track your API usage in PagerDuty's integration settings

!!! tip "Rate Limit Best Practices"
    - Implement retry logic with jitter to avoid thundering herd problems
    - Consider using PagerDuty's enterprise features for higher limits if needed
    - Monitor your integration's usage in PagerDuty's web interface

## Error Handling

The PagerDuty service returns standard HTTP status codes. Handle these appropriately in your applications:

### Success Responses

- __202 Accepted__: Event accepted for processing (asynchronous)

### Errors Before the Request

A send fails before any request for an event that PagerDuty would reject:

- A trigger event without a title or message
- An acknowledge or resolve event without a `dedup_key`
- A `dedup_key` over 255 characters
- A `details` value that is not a JSON object, or a `contexts` value that does not parse
- An encoded event over 512 KB (524,288 bytes), PagerDuty's size limit

Reading a service URL rejects each of these that the URL itself sets, such as `action=resolve` without a `dedup_key`.

### Error Responses

- __400 Bad Request__: Invalid request payload or parameters
  - Check integration key format (must be 32 hex characters)
  - Validate JSON structure for details/contexts
  - Ensure URLs are properly encoded

- __401 Unauthorized__: Invalid or missing integration key
  - Verify the integration key is correct
  - Check that the integration hasn't been disabled

- __403 Forbidden__: Integration lacks required permissions
  - Ensure the integration key has appropriate permissions
  - Check service configuration in PagerDuty

- __429 Too Many Requests__: Rate limit exceeded
  - Implement exponential backoff
  - Consider queuing messages

- __500 Internal Server Error__: PagerDuty service error
  - Retry with backoff
  - Check PagerDuty status page for outages

### Common Issues and Solutions

!!! tip "Integration Key Validation"
    Integration keys must be exactly 32 hexadecimal characters. Common mistakes include:
    - Using API keys instead of integration keys
    - Extra whitespace or characters
    - Using keys from disabled integrations

!!! tip "JSON Parameter Encoding"
    When passing complex parameters like `details` or `contexts` in URLs, ensure proper JSON encoding:

    **Correct:**

    ```bash
    details={"key":"value"}
    ```

    **Incorrect - missing quotes:**

    ```bash
    details={key:value}
    ```

!!! tip "Context Object Validation"
    Each context object must have a valid `type` field. Supported types are `link` and `image`.

## Code Examples

### Go Integration

#### Basic Incident Triggering Implementation

```go
package main

import (
    "log"
    "github.com/nicholas-fedor/shoutrrr"
)

func main() {
    url := "pagerduty:///INTEGRATION_KEY"

    err := shoutrrr.Send(url, "Database connection failed")
    if err != nil {
        log.Fatal(err)
    }
}
```

#### Advanced Configuration with Parameters

```go
package main

import (
    "context"
    "log"
    "github.com/nicholas-fedor/shoutrrr"
    "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func main() {
    url := "pagerduty:///INTEGRATION_KEY"

    // Trigger critical incident
    params := &types.Params{
        "severity": "critical",
        "source":   "production-db-01",
        "action":   "trigger",
        "details":  `{"database": "postgres", "error": "connection timeout"}`,
        "client":   "monitoring-system",
        "client_url": "https://monitoring.example.com/alerts/123",
    }

    err := shoutrrr.SendContext(context.Background(), url, "Production database is down", params)
    if err != nil {
        log.Fatal(err)
    }
}
```

#### Incident Management Workflow Implementation

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/nicholas-fedor/shoutrrr"
    "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func handleIncident() {
    baseURL := "pagerduty:///INTEGRATION_KEY"

    // 1. Trigger incident, naming it for the later events
    triggerParams := &types.Params{
        "severity":  "critical",
        "source":    "web-server-01",
        "action":    "trigger",
        "dedup_key": "web-server-01-unresponsive",
    }

    err := shoutrrr.SendContext(context.Background(), baseURL, "Web server unresponsive", triggerParams)
    if err != nil {
        log.Printf("Failed to trigger incident: %v", err)
        return
    }

    // Simulate investigation time
    time.Sleep(5 * time.Minute)

    // 2. Acknowledge incident
    ackParams := &types.Params{
        "action":    "acknowledge",
        "dedup_key": "web-server-01-unresponsive",
    }

    err = shoutrrr.SendContext(context.Background(), baseURL, "", ackParams)
    if err != nil {
        log.Printf("Failed to acknowledge incident: %v", err)
        return
    }

    // Simulate resolution time
    time.Sleep(15 * time.Minute)

    // 3. Resolve incident
    resolveParams := &types.Params{
        "action":    "resolve",
        "dedup_key": "web-server-01-unresponsive",
    }

    err = shoutrrr.SendContext(context.Background(), baseURL, "", resolveParams)
    if err != nil {
        log.Printf("Failed to resolve incident: %v", err)
        return
    }
}
```

#### Using Contexts for Rich Incident Details

```go
package main

import (
    "context"
    "log"
    "github.com/nicholas-fedor/shoutrrr"
    "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func sendRichIncident() {
    url := "pagerduty:///INTEGRATION_KEY"

    params := &types.Params{
        "severity": "error",
        "source":   "ci-pipeline",
        "action":   "trigger",
        "details":  `{
            "pipeline": "deploy-prod",
            "stage": "test",
            "commit": "abc123def",
            "branch": "main"
        }`,
        "contexts": `[
            {
                "type": "link",
                "href": "https://github.com/org/repo/commit/abc123def",
                "text": "View Commit"
            },
            {
                "type": "link",
                "href": "https://ci.example.com/pipelines/123",
                "text": "Pipeline Details"
            },
            {
                "type": "image",
                "src": "https://ci.example.com/screenshots/failure.png",
                "text": "Test Failure Screenshot"
            }
        ]`,
        "client":    "GitHub Actions",
        "client_url": "https://github.com/org/repo/actions/runs/456",
    }

    err := shoutrrr.SendContext(context.Background(), url, "CI/CD pipeline failed in production deployment", params)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Integration with Monitoring Systems

```go
package main

import (
    "context"
    "encoding/json"
    "github.com/nicholas-fedor/shoutrrr"
    "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// AlertData represents alert data from a monitoring system
type AlertData struct {
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Severity    string            `json:"severity"`
    Source      string            `json:"source"`
    Details     map[string]interface{} `json:"details"`
    DashboardURL string           `json:"dashboard_url"`
}

func sendMonitoringAlert(data AlertData) error {
    url := "pagerduty:///INTEGRATION_KEY"

    // Convert details to JSON string
    detailsJSON, err := json.Marshal(data.Details)
    if err != nil {
        return err
    }

    params := &types.Params{
        "title":      data.Title,
        "severity":   data.Severity,
        "source":     data.Source,
        "action":     "trigger",
        "details":    string(detailsJSON),
        "client":     "Custom Monitoring",
        "client_url": data.DashboardURL,
    }

    return shoutrrr.SendContext(context.Background(), url, data.Description, params)
}
```

## CLI Examples

### Basic Usage

Send a simple alert:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY' -m 'Server is down'
```

Send with severity:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?severity=critical' -m 'Production outage detected'
```

### Incident Lifecycle Management

Trigger incident:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?action=trigger&severity=critical&source=web-app&dedup_key=DEDUP_KEY' -m 'Application crashed'
```

Acknowledge incident:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?action=acknowledge&dedup_key=DEDUP_KEY' -m 'Acknowledged'
```

Resolve incident:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?action=resolve&dedup_key=DEDUP_KEY' -m 'Resolved'
```

### Advanced Parameters

With custom details:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?details={"component":"database","error":"connection_failed"}' -m 'Database connectivity issue'
```

With contexts:

```bash
shoutrrr send -u 'pagerduty:///INTEGRATION_KEY?contexts=[{"type":"link","href":"https://monitoring.example.com","text":"Monitoring Dashboard"}]' -m 'High CPU usage detected'
```

Full configuration:

```bash
shoutrrr send -u 'pagerduty://events.pagerduty.com/INTEGRATION_KEY?severity=warning&source=load-balancer&client=prometheus&client_url=https://prometheus.example.com/alerts' -m 'Load balancer health check failing'
```

### Integration with Scripts

Example script for PagerDuty alerts:

```bash
#!/bin/bash

INTEGRATION_KEY="eb243592faa24ba2a5511afdf565c889"
SEVERITY="${1:-error}"
SOURCE="${2:-unknown}"
MESSAGE="$3"

if [ -z "$MESSAGE" ]; then
    echo "Usage: $0 [severity] [source] message"
    exit 1
fi

URL="pagerduty:///$INTEGRATION_KEY?severity=$SEVERITY&source=$SOURCE"

shoutrrr send -u "$URL" -m "$MESSAGE"
```

## Best Practices

### Incident Management

#### Use Appropriate Severity Levels

- Reserve `critical` for issues requiring immediate attention
- Use `error` for actual problems, `warning` for potential issues
- Use `info` sparingly for informational events

#### Provide Meaningful Source Identifiers

- Use consistent, descriptive source names
- Include environment information (e.g., `prod-web-01`, `staging-db`)
- Avoid generic sources like "unknown" or "default"

#### Include Rich Context

- Always provide `client` and `client_url` for traceability
- Use `details` for structured data that can be queried
- Add relevant `contexts` to help responders

### Operational Considerations

#### Rate Limiting Awareness

- Monitor your API usage to avoid hitting limits
- Implement queuing for high-volume scenarios
- Use exponential backoff for retries

#### Error Handling Considerations

- Always check for errors when sending notifications
- Implement proper logging for debugging
- Handle rate limiting gracefully

#### Security

- Store integration keys securely (environment variables, secrets management)
- Rotate keys periodically
- Use least-privilege access

### Integration Patterns

#### Monitoring System Integration

- Map monitoring severity levels to PagerDuty severities
- Include monitoring system links in `client_url`
- Use consistent `source` naming conventions

#### CI/CD Pipeline Integration

- Trigger incidents for deployment failures
- Include build links and commit information
- Use appropriate severity based on environment

#### Application Integration

- Include error codes, stack traces in `details`
- Provide links to logs and monitoring
- Use structured logging for better incident data

### Maintenance

#### Regular Testing

- Test integrations regularly to ensure they work
- Verify integration keys are valid
- Test different severity levels and parameters

#### Documentation

- Document your PagerDuty integrations
- Maintain runbooks for common incident types
- Keep contact information current

#### Monitoring

- Monitor PagerDuty integration health
- Set up alerts for integration failures
- Review incident response times and effectiveness

By following these best practices, you can ensure reliable, effective incident management that helps your team respond quickly and effectively to system issues.
