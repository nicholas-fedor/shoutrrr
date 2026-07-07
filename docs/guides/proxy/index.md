# Proxy Setup

## Overview

Shoutrrr supports proxying HTTP requests for notification services, allowing you to route traffic through a proxy server. This can be configured using an environment variable or by customizing the HTTP client in code.

**For per-sender egress control and SSRF protection (recommended for untrusted notification URLs), use a custom `http.Client` via `SenderOptions`.**

## Usage

### Environment Variable

Set the `HTTP_PROXY` environment variable to the proxy URL.
This applies to all HTTP-based services used by Shoutrrr that rely on the default transport.

```bash title="Set HTTP_PROXY Environment Variable"
export HTTP_PROXY="socks5://localhost:1337"
```

!!! Note
    This is a process-global setting and affects `http.DefaultClient` and clients that inherit `http.DefaultTransport`.
    It is not suitable for per-sender SSRF controls.

### Custom HTTP Client (Per-Sender, Recommended for SSRF/Egress Control)

Supply a custom `*http.Client` (with custom `Transport`, `DialContext`, TLS config, etc.) when creating a sender or router. The client is injected into services that support it and used for all their outbound HTTP.

Use `shoutrrr.NewSenderWithOptions` (or `CreateSenderWithOptions`, `router.NewWithOptions`).

```go title="Configure Custom HTTP Client for SSRF Protection"
package main

import (
 "context"
 "fmt"
 "log"
 "net"
 "net/http"
 "time"

 "github.com/nicholas-fedor/shoutrrr"
 "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// isAllowedHost is an example SSRF guard. Implement your own policy.
func isAllowedHost(host string) bool {
 // Reject loopback, private, link-local, etc. Adjust to your needs.
 ip := net.ParseIP(host)
 if ip == nil {
  // For hostnames you may resolve or apply allow-list here.
  return true
 }
 if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
  return false
 }
 return true
}

func main() {
 // Custom Transport with DialContext that performs egress/SSRF checks.
 transport := &http.Transport{
  DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
   host, _, err := net.SplitHostPort(addr)
   if err != nil {
    host = addr // no port
   }
   if !isAllowedHost(host) {
    return nil, &net.OpError{Op: "dial", Net: network, Addr: nil, Err: fmt.Errorf("destination blocked by egress policy")}
   }
   d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
   return d.DialContext(ctx, network, addr)
  },
  // Proxy: http.ProxyFromEnvironment, // opt-in if you also want env proxies for this client
  ForceAttemptHTTP2:     true,
  MaxIdleConns:          100,
  IdleConnTimeout:       90 * time.Second,
  TLSHandshakeTimeout:   10 * time.Second,
  ExpectContinueTimeout: 1 * time.Second,
 }

 customClient := &http.Client{
  Transport: transport,
  Timeout:   60 * time.Second,
 }

 opts := types.SenderOptions{
  HTTPClient: customClient,
  // Timeout: 30 * time.Second, // optional per-router override
 }

 url := "discord://abc123@123456789"
 sender, err := shoutrrr.NewSenderWithOptions(nil, opts, url)
 if err != nil {
  log.Fatal(err)
 }

 if errs := sender.Send("Hello via custom client!", nil); len(errs) > 0 {
  for _, e := range errs {
   log.Println("Error:", e)
  }
 }
}
```

**Notes on custom clients:**

- A non-nil `SenderOptions.HTTPClient` is propagated by the router to services implementing `types.HTTPClientSetter`.
- Custom clients usually bypass `HTTP_PROXY`/`HTTPS_PROXY` unless their `Transport.Proxy` is configured to consult the environment.
- All default timeouts/TLS behavior is preserved when no custom client is supplied.
- The same client instance is reused for the lifetime of the sender/router.

## Examples

<!-- markdownlint-disable -->
### Using Environment Variable for Proxy (Global)

!!! Example
    ```bash title="Set Proxy and Send Notification"
    export HTTP_PROXY="socks5://localhost:1337"
    shoutrrr send --url "discord://abc123@123456789" --message "Hello via proxy!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Using Custom HTTP Client in Go (Per-Sender / SSRF Control)

!!! Example
    ```go title="Send Notification with Custom Client (SSRF-safe)"
    package main

    import (
        "context"
        "log"
        "net"
        "net/http"
        "time"

        "github.com/nicholas-fedor/shoutrrr"
        "github.com/nicholas-fedor/shoutrrr/pkg/types"
    )

    func isAllowedHost(host string) bool {
        ip := net.ParseIP(host)
        if ip == nil {
            return true
        }
        return !ip.IsLoopback() && !ip.IsPrivate()
    }

    func main() {
        transport := &http.Transport{
            DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
                host, _, _ := net.SplitHostPort(addr)
                if !isAllowedHost(host) {
                    return nil, context.DeadlineExceeded
                }
                return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, addr)
            },
        }
        custom := &http.Client{Transport: transport}

        sender, err := shoutrrr.NewSenderWithOptions(nil, types.SenderOptions{HTTPClient: custom}, "discord://abc123@123456789")
        if err != nil {
            log.Fatal(err)
        }
        if errs := sender.Send("Hello via custom egress-controlled client!", nil); len(errs) > 0 {
            for _, e := range errs {
                log.Println(e)
            }
        }
    }
    ```

    ```text title="Expected Output (Success)"
    (No output on success)
    ```

    ```text title="Expected Output (Error)"
    Error: failed to send message: unexpected response status code
    ```
<!-- markdownlint-restore -->

## Notes

- **Environment Variable**: `HTTP_PROXY` supports protocols like `http`, `https`, or `socks5`. It affects all HTTP-based services globally.
- **Custom HTTP Client**: Provides fine-grained control over proxy settings, suitable for Go applications requiring specific transport configurations.
- **Service Compatibility**: Ensure the proxy supports the protocol used by the service (e.g., HTTPS for Discord, SMTP).
- **Timeouts**: The custom client example includes a 30-second dial timeout and 10-second TLS handshake timeout, adjustable as needed.
