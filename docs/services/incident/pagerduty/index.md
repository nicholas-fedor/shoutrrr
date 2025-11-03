# PagerDuty

## Overview

PagerDuty is a digital operations management platform that helps teams respond to incidents and outages. The PagerDuty service integrates with PagerDuty's Events API v2 to create, acknowledge, and resolve incidents programmatically. This service enables automated incident management workflows, allowing you to trigger alerts when issues are detected, acknowledge them when investigation begins, and resolve them when problems are fixed.

The service supports rich incident context including custom details, links to related resources, images, and client information to provide comprehensive incident information to on-call responders.

## URL Format

--8<-- "docs/services/incident/pagerduty/config.md"

### URL Components

The PagerDuty service URL follows this structure:

```uri
pagerduty://[host[:port]]/integration-key[?query-parameters]
```

**Components:**

- **Scheme**: `pagerduty://` - Identifies the service type
- **Host** (optional): PagerDuty API hostname (default: `events.pagerduty.com`)
- **Port** (optional): API port number (default: `443` for HTTPS)
- **Integration Key**: 32-character hexadecimal string that routes events to your PagerDuty service
- **Query Parameters** (optional): Additional configuration parameters

### Examples

!!! example "Default configuration"
    ```url
    pagerduty:///eb243592faa24ba2a5511afdf565c889
    ```
    Uses default host (`events.pagerduty.com`) and port (`443`)

!!! example "Custom host"
    ```url
    pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889
    ```
    Explicitly specifies the PagerDuty Events API host

!!! example "With parameters"
    ```url
    pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=critical&source=production-db
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

1. **Log in to PagerDuty**
   - Access your PagerDuty account at [pagerduty.com](https://pagerduty.com)

2. **Navigate to Services**
   - Go to **Services** → **Service Directory** in the main navigation

3. **Create or Select a Service**
   - Either create a new service or select an existing one
   - If creating new: Click **+ New Service**, give it a name, and select an escalation policy

4. **Add Events API v2 Integration**
   - In the service details, click **Integrations** tab
   - Click **+ Add another integration**
   - Search for and select **Events API v2**
   - Give the integration a name (e.g., "Shoutrrr Integration")
   - Click **Create Integration**

5. **Copy Integration Key**
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
|-----------|------|-------------|----------|---------|--------------|
| `severity` | string | The perceived severity of the incident | No | `error` | `info`, `warning`, `error`, `critical` |
| `source` | string | The unique location of the affected system | No | `default` | Any string identifier |
| `action` | string | The type of event to send | No | `trigger` | `trigger`, `acknowledge`, `resolve` |

### Additional Parameters

| Parameter | Type | Description | Required | Default | Valid Values |
|-----------|------|-------------|----------|---------|--------------|
| `details` | object | Additional custom key-value pairs for the incident | No | - | JSON object or URL-encoded string |
| `contexts` | array | Additional context such as links or images | No | - | JSON array of context objects |
| `client` | string | Name of the monitoring client/integration | No | - | Any string |
| `client_url` | string | URL of the monitoring client dashboard | No | - | Valid URL |

### Parameter Details

#### Severity Levels

- **`info`**: Informational events that don't require immediate action
- **`warning`**: Potential issues that should be monitored
- **`error`**: Actual problems affecting system operation
- **`critical`**: Urgent issues requiring immediate response

#### Event Action Items

- **`trigger`**: Creates a new incident or updates an existing one
- **`acknowledge`**: Acknowledges an incident (stops escalation)
- **`resolve`**: Resolves an incident (closes it)

#### Details Parameter

The `details` parameter accepts a JSON object with custom key-value pairs:

```json
{
  "component": "database",
  "version": "2.1.0",
  "error_code": "ECONNREFUSED",
  "affected_users": 150
}
```

#### Contexts Parameter

The `contexts` parameter accepts a JSON array of context objects. Each context must have a `type` field:

**Link contexts:**

```json
{
  "type": "link",
  "href": "https://example.com/logs",
  "text": "View Application Logs"
}
```

**Image contexts:**

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
    pagerduty:///eb243592faa24ba2a5511afdf565c889
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889' -m 'Database connection lost'
    ```

!!! example "Critical incident with source"
    ```url
    pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=critical&source=prod-db-01
    ```
    ```bash
    shoutrrr send -u 'pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=critical&source=prod-db-01' -m 'Production database is unresponsive'
    ```

### Incident Management Workflow

!!! example "Acknowledge incident"
    ```url
    pagerduty:///eb243592faa24ba2a5511afdf565c889?action=acknowledge
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?action=acknowledge' -m 'Engineer has acknowledged the incident and is investigating'
    ```

!!! example "Resolve incident"
    ```url
    pagerduty:///eb243592faa24ba2a5511afdf565c889?action=resolve
    ```
    ```bash
    shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?action=resolve' -m 'Database connection restored, incident resolved'
    ```

### Advanced Examples

!!! example "With custom details"
    ```url
    pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=warning&source=monitoring&details={"component":"api","version":"1.2.3","response_time":"5.2s"}
    ```
    ```bash
    shoutrrr send -u 'pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=warning&source=monitoring&details={"component":"api","version":"1.2.3","response_time":"5.2s"}' -m 'API response time degraded'
    ```

!!! example "With contexts and client info"
    ```url
    pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=error&source=ci-pipeline&contexts=[{"type":"link","href":"https://github.com/org/repo/actions/runs/123","text":"GitHub Actions Run"},{"type":"image","src":"https://example.com/failure-screenshot.png","text":"Build Failure"}]&client=jenkins&client_url=https://ci.example.com/job/123
    ```
    ```bash
    shoutrrr send -u 'pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=error&source=ci-pipeline&contexts=[{"type":"link","href":"https://github.com/org/repo/actions/runs/123","text":"GitHub Actions Run"}]&client=jenkins&client_url=https://ci.example.com/job/123' -m 'CI/CD pipeline failed'
    ```

## Event Actions

The PagerDuty service supports three primary event actions that correspond to the incident lifecycle:

### Trigger (`action=trigger`)

Creates a new incident or updates an existing one with new information. This is the default action.

**Use cases:**

- System failures or outages
- Performance degradation
- Security alerts
- Application errors

**Behavior:**

- If no incident exists for the service, creates a new one
- If an incident exists, adds a new trigger event (may escalate if unacknowledged)
- Includes all provided context, details, and parameters

### Acknowledge (`action=acknowledge`)

Acknowledges an existing incident, stopping escalation and indicating that someone is actively working on it.

**Use cases:**

- Engineer has started investigation
- Issue has been triaged
- Response team is assembled

**Behavior:**

- Stops incident escalation
- Records acknowledgment timestamp
- Can include update notes in the message

### Resolve (`action=resolve`)

Resolves an incident, marking it as completed and closing the alert cycle.

**Use cases:**

- Issue has been fixed
- Service has been restored
- Incident is no longer relevant

**Behavior:**

- Closes the incident
- Records resolution timestamp
- Prevents further notifications for this incident

!!! note "Incident Deduplication"
    PagerDuty automatically deduplicates incidents based on the integration key and incident details. Multiple triggers for the same issue will update the existing incident rather than create duplicates.

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

**Link Contexts:**

- Direct responders to relevant resources
- Include runbooks, documentation, or monitoring dashboards
- Support both internal and external links

**Image Contexts:**

- Attach screenshots or graphs
- Show error visualizations or system diagrams
- Include clickable links to full-size images

### Client Information

The `client` and `client_url` parameters identify the monitoring system:

- **`client`**: Name of your monitoring tool (e.g., "Prometheus", "DataDog", "Custom Monitor")
- **`client_url`**: Direct link to the monitoring dashboard or alert details

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

The PagerDuty Events API v2 enforces rate limits to ensure system stability:

- **300 requests per minute** per integration key
- Rate limits are enforced per integration key, not per service
- Exceeding limits returns HTTP 429 (Too Many Requests)

### Handling Rate Limits

When rate limits are exceeded:

1. **Implement backoff**: Use exponential backoff (start with 1 minute, double each retry)
2. **Queue messages**: Buffer notifications during high-volume periods
3. **Use multiple integrations**: Distribute load across multiple integration keys if needed
4. **Monitor usage**: Track your API usage in PagerDuty's integration settings

!!! tip "Rate Limit Best Practices"
    - Implement retry logic with jitter to avoid thundering herd problems
    - Consider using PagerDuty's enterprise features for higher limits if needed
    - Monitor your integration's usage in PagerDuty's web interface

## Error Handling

The PagerDuty service returns standard HTTP status codes. Handle these appropriately in your applications:

### Success Responses

- **200 OK**: Event processed successfully
- **202 Accepted**: Event accepted for processing (asynchronous)

### Error Responses

- **400 Bad Request**: Invalid request payload or parameters
  - Check integration key format (must be 32 hex characters)
  - Validate JSON structure for details/contexts
  - Ensure URLs are properly encoded

- **401 Unauthorized**: Invalid or missing integration key
  - Verify the integration key is correct
  - Check that the integration hasn't been disabled

- **403 Forbidden**: Integration lacks required permissions
  - Ensure the integration key has appropriate permissions
  - Check service configuration in PagerDuty

- **429 Too Many Requests**: Rate limit exceeded
  - Implement exponential backoff
  - Consider queuing messages

- **500 Internal Server Error**: PagerDuty service error
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

    Correct:

    ```bash
    details={"key":"value"}
    ```

    Incorrect - missing quotes:

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
    "github.com/containrrr/shoutrrr"
)

func main() {
    url := "pagerduty:///eb243592faa24ba2a5511afdf565c889"
    
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
    "log"
    "github.com/containrrr/shoutrrr"
    "github.com/containrrr/shoutrrr/pkg/types"
)

func main() {
    url := "pagerduty:///eb243592faa24ba2a5511afdf565c889"
    
    // Trigger critical incident
    params := &types.Params{
        "severity": "critical",
        "source":   "production-db-01",
        "action":   "trigger",
        "details":  `{"database": "postgres", "error": "connection timeout"}`,
        "client":   "monitoring-system",
        "client_url": "https://monitoring.example.com/alerts/123",
    }
    
    err := shoutrrr.Send(url, "Production database is down", params)
    if err != nil {
        log.Fatal(err)
    }
}
```

#### Incident Management Workflow Implementation

```go
package main

import (
    "log"
    "time"
    "github.com/containrrr/shoutrrr"
    "github.com/containrrr/shoutrrr/pkg/types"
)

func handleIncident() {
    baseURL := "pagerduty:///eb243592faa24ba2a5511afdf565c889"
    
    // 1. Trigger incident
    triggerParams := &types.Params{
        "severity": "critical",
        "source":   "web-server-01",
        "action":   "trigger",
    }
    
    err := shoutrrr.Send(baseURL, "Web server unresponsive", triggerParams)
    if err != nil {
        log.Printf("Failed to trigger incident: %v", err)
        return
    }
    
    // Simulate investigation time
    time.Sleep(5 * time.Minute)
    
    // 2. Acknowledge incident
    ackParams := &types.Params{
        "action": "acknowledge",
    }
    
    err = shoutrrr.Send(baseURL, "Engineer investigating server issue", ackParams)
    if err != nil {
        log.Printf("Failed to acknowledge incident: %v", err)
        return
    }
    
    // Simulate resolution time
    time.Sleep(15 * time.Minute)
    
    // 3. Resolve incident
    resolveParams := &types.Params{
        "action": "resolve",
    }
    
    err = shoutrrr.Send(baseURL, "Web server restarted successfully", resolveParams)
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
    "log"
    "github.com/containrrr/shoutrrr"
    "github.com/containrrr/shoutrrr/pkg/types"
)

func sendRichIncident() {
    url := "pagerduty:///eb243592faa24ba2a5511afdf565c889"
    
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
    
    err := shoutrrr.Send(url, "CI/CD pipeline failed in production deployment", params)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Integration with Monitoring Systems

```go
package main

import (
    "encoding/json"
    "log"
    "github.com/containrrr/shoutrrr"
    "github.com/containrrr/shoutrrr/pkg/types"
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
    url := "pagerduty:///eb243592faa24ba2a5511afdf565c889"
    
    // Convert details to JSON string
    detailsJSON, err := json.Marshal(data.Details)
    if err != nil {
        return err
    }
    
    params := &types.Params{
        "severity":   data.Severity,
        "source":     data.Source,
        "action":     "trigger",
        "details":    string(detailsJSON),
        "client":     "Custom Monitoring",
        "client_url": data.DashboardURL,
    }
    
    return shoutrrr.Send(url, data.Title+": "+data.Description, params)
}
```

## CLI Examples

### Basic Usage

Send a simple alert:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889' -m 'Server is down'
```

Send with severity:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?severity=critical' -m 'Production outage detected'
```

### Incident Lifecycle Management

Trigger incident:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?action=trigger&severity=critical&source=web-app' -m 'Application crashed'
```

Acknowledge incident:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?action=acknowledge' -m 'DevOps team is investigating'
```

Resolve incident:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?action=resolve' -m 'Application restarted successfully'
```

### Advanced Parameters

With custom details:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?details={"component":"database","error":"connection_failed"}' -m 'Database connectivity issue'
```

With contexts:

```bash
shoutrrr send -u 'pagerduty:///eb243592faa24ba2a5511afdf565c889?contexts=[{"type":"link","href":"https://monitoring.example.com","text":"Monitoring Dashboard"}]' -m 'High CPU usage detected'
```

Full configuration:

```bash
shoutrrr send -u 'pagerduty://events.pagerduty.com/eb243592faa24ba2a5511afdf565c889?severity=warning&source=load-balancer&client=prometheus&client_url=https://prometheus.example.com/alerts' -m 'Load balancer health check failing'
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

URL="pagerduty:///integration-key?severity=$SEVERITY&source=$SOURCE"

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
