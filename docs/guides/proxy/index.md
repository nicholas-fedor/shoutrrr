# Proxy Setup

## Overview

Shoutrrr supports proxying HTTP requests for notification services, allowing you to route traffic through a proxy server. This can be configured using an environment variable or by customizing the HTTP client in code.

## Usage

### Environment Variable

Set the `HTTP_PROXY` environment variable to the proxy URL. This applies to all HTTP-based services used by Shoutrrr.

```bash title="Set HTTP_PROXY Environment Variable"
export HTTP_PROXY="socks5://localhost:1337"
```

### Custom HTTP Client

Override the default HTTP client in your Go code to configure a proxy with specific transport settings.

```go title="Configure Custom HTTP Client with Proxy"
import (
    "log"
    "net/http"
    "net/url"
    "time"
)

proxyURL, err := url.Parse("socks5://localhost:1337")
if err != nil {
    log.Fatalf("Error parsing proxy URL: %v", err)
}

http.DefaultClient.Transport = &http.Transport{
    Proxy: http.ProxyURL(proxyURL),
    DialContext: (&net.Dialer{
        Timeout:   30 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
    ForceAttemptHTTP2:     true,
    MaxIdleConns:          100,
    IdleConnTimeout:       90 * time.Second,
    TLSHandshakeTimeout:   10 * time.Second,
    ExpectContinueTimeout: 1 * time.Second,
}
```

## Examples

<!-- markdownlint-disable -->
### Using Environment Variable for Proxy

!!! Example
    ```bash title="Set Proxy and Send Notification"
    export HTTP_PROXY="socks5://localhost:1337"
    shoutrrr send --url "discord://abc123@123456789" --message "Hello via proxy!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Using Custom HTTP Client in Go

!!! Example
    ```go title="Send Notification with Proxy"
    package main

    import (
        "log"
        "net/http"
        "net/url"
        "time"
        "github.com/nicholas-fedor/shoutrrr"
    )

    func main() {
        proxyURL, err := url.Parse("socks5://localhost:1337")
        if err != nil {
            log.Fatalf("Error parsing proxy URL: %v", err)
        }

        http.DefaultClient.Transport = &http.Transport{
            Proxy: http.ProxyURL(proxyURL),
            DialContext: (&net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
            ForceAttemptHTTP2:     true,
            MaxIdleConns:          100,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:   10 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
        }

        url := "discord://abc123@123456789"
        errs := shoutrrr.Send(url, "Hello via proxy!")
        if len(errs) > 0 {
            for _, err := range errs {
                log.Println("Error:", err)
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
