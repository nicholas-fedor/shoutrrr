package teams

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Parts of the Power Automate workflow webhook URLs that the "When a Teams
// webhook request is received" trigger issues.
//
// Power Platform URLs look like
// https://*.environment.api.powerplatform.com/powerautomate/automations/direct/.../workflows/....
// Azure Logic Apps URLs look like https://*.logic.azure.com/workflows/..., and
// https://*.logic.azure.us/workflows/... and https://*.logic.azure.cn/workflows/...
// in the US Government and China clouds.
//
// Power Automate flows left logic.azure.com on 2025-11-30, and their old URLs
// on that domain stopped working. Azure Logic Apps workflows still use it, and
// their URLs have the same form, so the domain stays accepted. Azure Germany
// (logic.azure.de) closed on 2021-10-29.
//
// References:
//   - https://learn.microsoft.com/en-us/troubleshoot/power-platform/power-automate/flow-run-issues/triggers-troubleshoot?tabs=new-designer#changes-to-http-or-teams-webhook-trigger-flows
//   - https://learn.microsoft.com/en-us/azure/logic-apps/logic-apps-http-endpoint
//   - https://learn.microsoft.com/en-us/entra/identity-platform/authentication-national-cloud
const (
	// powerPlatformHostSuffix ends the host of a Power Platform workflow URL.
	powerPlatformHostSuffix = ".environment.api.powerplatform.com"
	// directPathPrefix starts the path of a Power Platform workflow URL. Routing
	// segments that Microsoft inserts for its ingress gateway, such as "cu/04/",
	// may follow it before "workflows/".
	directPathPrefix = "/powerautomate/automations/direct/"
	// workflowsSegment is the path segment, with its slash, that the path of a
	// workflow URL reaches.
	workflowsSegment = "workflows/"
	// signatureKey is the query key of the shared access signature that
	// authorizes a call to a workflow that anyone can trigger.
	signatureKey = "sig"
)

// logicAppsHostSuffixes end the hosts of Azure Logic Apps workflow URLs in
// the global, US Government, and China clouds.
var logicAppsHostSuffixes = []string{".logic.azure.com", ".logic.azure.us", ".logic.azure.cn"}

// ValidateWebhookURL checks that the webhook URL is a Power Automate workflow
// webhook URL that Shoutrrr can send to.
//
// The URL is read as an HTTP request reads it. Its host must be a Power
// Platform or an Azure Logic Apps workflow host, with a port from 1 to 65535
// if it has one, and its path must lead to "workflows/".
// Its query must carry a signature in a non-empty sig parameter. A trigger that
// anyone can call authorizes requests by that signature. A URL without one
// belongs to a trigger that requires sign-in, which Shoutrrr cannot provide.
// The rest of the URL is not checked.
//
// Parameters:
//   - rawURL: the webhook URL.
//
// Returns:
//   - error: an error wrapping [ErrInvalidWebhookURL] for a URL that fails
//     these checks. The error never includes the URL, whose signature is a
//     secret.
func ValidateWebhookURL(rawURL string) error {
	webhookURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: the URL does not parse", ErrInvalidWebhookURL)
	}

	if !isWorkflowURL(webhookURL) {
		return fmt.Errorf("%w: not a Power Automate workflow webhook URL", ErrInvalidWebhookURL)
	}

	query, err := util.ParseURLQuery(webhookURL)
	if err != nil {
		return fmt.Errorf("%w: the URL's query does not parse", ErrInvalidWebhookURL)
	}

	if query.Get(signatureKey) == "" {
		return fmt.Errorf("%w: the URL has no sig parameter", ErrInvalidWebhookURL)
	}

	return nil
}

// isWorkflowURL reports whether a URL has the scheme, host, and path of a
// Power Automate workflow webhook URL.
//
// Parameters:
//   - webhookURL: the parsed webhook URL.
//
// Returns:
//   - bool: true for an https URL without userinfo whose host is a workflow
//     host and whose path leads to "workflows/" as that host's URLs do.
func isWorkflowURL(webhookURL *url.URL) bool {
	if webhookURL.Scheme != "https" || webhookURL.User != nil || webhookURL.Opaque != "" {
		return false
	}

	hostname, _, err := util.SplitURLHost(webhookURL)
	if err != nil {
		return false
	}

	// Hostnames are case-insensitive, and url.Parse keeps their case.
	hostname = strings.ToLower(hostname)
	path := webhookURL.EscapedPath()
	rest, direct := strings.CutPrefix(path, directPathPrefix)

	switch {
	case hasSubdomain(hostname, powerPlatformHostSuffix):
		return direct && reachesWorkflows(rest)
	case slices.ContainsFunc(logicAppsHostSuffixes, func(suffix string) bool { return hasSubdomain(hostname, suffix) }):
		if direct {
			return reachesWorkflows(rest)
		}

		return strings.HasPrefix(path, "/"+workflowsSegment)
	default:
		return false
	}
}

// hasSubdomain reports whether a hostname is a subdomain of a domain.
//
// Parameters:
//   - hostname: the lowercase hostname.
//   - suffix: the domain, with its leading dot.
//
// Returns:
//   - bool: true when the hostname ends with the suffix after one or more
//     labels of letters, digits, and hyphens.
func hasSubdomain(hostname, suffix string) bool {
	labels, found := strings.CutSuffix(hostname, suffix)
	if !found {
		return false
	}

	for label := range strings.SplitSeq(labels, ".") {
		if !isLDHToken(label) {
			return false
		}
	}

	return true
}

// reachesWorkflows reports whether a path, after the direct path prefix,
// reaches the workflows segment through routing segments only.
//
// Parameters:
//   - path: the rest of the path after the direct path prefix.
//
// Returns:
//   - bool: true when the path starts with "workflows/" after zero or more
//     segments of letters, digits, and hyphens.
func reachesWorkflows(path string) bool {
	for !strings.HasPrefix(path, workflowsSegment) {
		segment, rest, found := strings.Cut(path, "/")
		if !found || !isLDHToken(segment) {
			return false
		}

		path = rest
	}

	return true
}

// isLDHToken reports whether a hostname label or a path segment holds only
// ASCII letters, digits, and hyphens.
//
// Parameters:
//   - token: the label or segment, without dots or slashes.
//
// Returns:
//   - bool: true for a non-empty token of ASCII letters, digits, and hyphens.
func isLDHToken(token string) bool {
	if token == "" {
		return false
	}

	for _, c := range []byte(token) {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-':
		default:
			return false
		}
	}

	return true
}
