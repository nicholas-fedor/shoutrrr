# Using the Shoutrrr Package

## Overview

The Shoutrrr Go package (`github.com/nicholas-fedor/shoutrrr`) enables sending notifications to various services (e.g., `discord`, `slack`, `telegram`, `smtp`, etc.) using service URLs. It provides two primary methods: a direct `Send` function for simple use cases and a `Sender` struct for advanced scenarios with multiple URLs, message queuing, and parameter customization.

### Minimum Supported Go Version

Projects importing Shoutrrr are expected to follow the latest Go minor and/or patch semantic versions; ergo, Shoutrrr follows the latest minor Go version, i.e. 1.27. (Go refers to minor semantic version releases as major releases.)

Go release information can be reviewed here: <https://go.dev/doc/devel/release>

## Usage

```go title="Go Import Statement"
import "github.com/nicholas-fedor/shoutrrr"
```

### Direct Send

Sends a notification to a single service URL.

- **Function**: `shoutrrr.Send(url string, message string) error`
- **Behavior**: Initializes a service from the provided URL, sends the message, and returns any error.

!!! Example
    ```go title="Send to a Single Slack URL"
    url := "slack://token-a/token-b/token-c"
    err := shoutrrr.Send(url, "Hello, Slack!")
    if err != nil {
        fmt.Println("Error:", err)
    }
    ```

### Sender

Creates a `Sender` (`*ServiceRouter`) to manage multiple service URLs, support message queuing, and allow parameter customization.

- **Function**: `shoutrrr.CreateSender(urls ...string) (*ServiceRouter, error)`
- **Methods**:
  - `Send(message string, params *types.Params) []error`: Sends a message to all configured services.
  - `SendItems(items []types.MessageItem, params types.Params) []error`: Sends structured message items to services that support rich formatting.
  - `SendAsync(message string, params *types.Params) chan error`: Sends a message asynchronously and returns a channel of errors.
  - `Enqueue(message string, v ...interface{})`: Queues a formatted message for later sending.
  - `Flush(params *types.Params)`: Sends all queued messages and resets the queue.
- **Behavior**: Deduplicates URLs, initializes services, and supports asynchronous sending with a 10-second timeout per service.

!!! Example
    ```go title="Create Sender with Multiple URLs"
    urls := []string{
        "slack://token-a/token-b/token-c",
        "telegram://110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw@telegram?channels=@mychannel",
    }
    sender, err := shoutrrr.CreateSender(urls...)
    if err != nil {
        log.Fatal(err)
    }
    params := types.Params{}
    params.SetTitle("Test Notification")
    errs := sender.Send("Hello, world!", &params)
    if len(errs) > 0 {
        for _, err := range errs {
            fmt.Println("Error:", err)
        }
    }
    ```

### Message Queuing

Allows queuing messages for deferred sending, useful for aggregating notifications during a process.

- Queues messages with `Enqueue` and sends them with `Flush`. Queued messages use the `params` provided during `Flush`.

<!-- markdownlint-disable -->
!!! Example
    ```go title="Queue and Flush Notifications"
    url := "discord://abc123@123456789"
    sender, err := shoutrrr.CreateSender(url)
    if err != nil {
        log.Fatal(err)
    }
    defer sender.Flush(nil)

    sender.Enqueue("Started doing work")
    if err := doWork(); err != nil {
        sender.Enqueue("Error: %v", err)
        return
    }
    sender.Enqueue("Work completed successfully!")
    ```
<!-- markdownlint-restore -->

### SendItems and RichSender

Sends structured message items to services that support rich formatting.
Services implementing `types.RichSender` receive the full `[]types.MessageItem` slice, preserving level, fields, and file attachments.
Services that do not implement `RichSender` fall back to plain text automatically.

- **Method**: `SendItems(items []types.MessageItem, params types.Params) []error`
- **Behavior**: Dispatches to `ContextAttachmentSender` if available, otherwise `RichSender`, otherwise falls back to `Send` with plain text.

!!! Example
    ```go title="Send Structured Message Items"
    items := []types.MessageItem{
        {Text: "Deployment complete", Level: types.Info},
        {Text: "Rollback available", Level: types.Warning},
    }
    sender, err := shoutrrr.CreateSender("discord://webhook")
    if err != nil {
        log.Fatal(err)
    }
    errs := sender.SendItems(items, types.Params{})
    ```

### Context Propagation

Services that implement `types.ContextSender` or `types.ContextAttachmentSender` receive a `context.Context` derived from the router's base context with a per-service timeout.
This enables cancellation and deadline propagation without changing the existing `Sender` or `RichSender` contracts.

### Per-Target Errors

`*ServiceRouter.Send`, `*ServiceRouter.SendAsync`, `*ServiceRouter.SendItems`, and `*ServiceRouter.Route` return one error per unique configured target, in the deduplicated target order produced by `CreateSender`. Each error is wrapped in `*types.TargetError`, which carries the service URL/ID and supports `errors.Unwrap`, `errors.Is`, and `errors.As`.

!!! Example
    ```go title="Handle Per-Target Errors"
    errs := sender.Send("deploy complete", nil)
    for i, err := range errs {
        if err == nil {
            continue
        }
        var targetErr *types.TargetError
        if errors.As(err, &targetErr) {
            log.Printf("failed to send to %s: %v", targetErr.URL, targetErr.Err)
        }
    }
    ```

### Message Levels

Services that support severity or priority can receive a semantic level through the `level` param.
Shoutrrr defines five levels: `Unknown`, `Debug`, `Info`, `Warning`, and `Error`.
The default is `Info`.

!!! Example
    ```go title="Set Message Level"
    params := types.Params{}
    params.SetLevel(types.Warning)
    params.SetTitle("Disk usage high")
    sender, err := shoutrrr.CreateSender("discord://webhook")
    if err != nil {
        log.Fatal(err)
    }
    errs := sender.Send("Root partition at 92%", &params)
    ```

### Format Conversion

Converts message bodies between supported formats.

```go title="Convert Message Format"
import "github.com/nicholas-fedor/shoutrrr/pkg/format"

body, err := format.ConvertFormat("Hello **world**", "markdown", "text")
```

Supported conversions: `text` ↔ `markdown` ↔ `html`.

### Service Discovery

Enumerates or checks available notification services without constructing a router.

```go title="Discover Available Services"
import "github.com/nicholas-fedor/shoutrrr/pkg/services"

for _, schema := range services.SupportedSchemas() {
    fmt.Println(schema)
}

if services.SupportsSchema("discord") {
    // ...
}
```

## Examples

<!-- markdownlint-disable -->
### Send with Parameters and Error Handling

!!! Example
    ```go title="Send with Title and Error Handling"
    url := "discord://abc123@123456789"
    sender, err := shoutrrr.CreateSender(url)
    if err != nil {
        log.Fatal(err)
    }
    params := types.Params{}
    params.SetTitle("Alert")
    errs := sender.Send("System alert!", &params)
    if len(errs) > 0 {
        for _, err := range errs {
            fmt.Println("Error:", err)
        }
    }
    ```

    ```text title="Expected Output (Success)"
    (No output on success)
    ```

    ```text title="Expected Output (Error)"
    Error: failed to send message: unexpected response status code
    ```

### Send to Multiple Services with Queuing

!!! Example
    ```go title="Queue Messages for Multiple Services"
    urls := []string{
        "slack://token-a/token-b/token-c",
        "discord://abc123@123456789",
    }
    sender, err := shoutrrr.CreateSender(urls...)
    if err != nil {
        log.Fatal(err)
    }
    start := time.Now()
    params := types.Params{}
    params.SetTitle("Task Summary")
    defer sender.Flush(&params)
    sender.Enqueue("Task started")
    time.Sleep(time.Second)
    sender.Enqueue("Task finished in %v", time.Now().Sub(start))
    ```

    ```text title="Expected Output (Success)"
    (No output on success)
    ```

    ```text title="Expected Output (Error)"
    Error: failed to initialize service: invalid URL format
    ```
<!-- markdownlint-restore -->

## Notes

- **Error Handling**: `Send` returns a single error. `Sender.Send`, `SendItems`, and `SendAsync` return one error per service. Check `len(errs) > 0` to handle failures. Each error is wrapped in `*types.TargetError` with the service URL.
- **Parameters**: `params` is a `*types.Params` value for `Send`, `SendAsync`, and `Flush`. `SendItems` accepts `types.Params` by value. Use setter methods such as `SetTitle`, `SetMessage`, and `SetLevel` to configure service-specific options. Use `shoutrrr docs` to view supported parameters for each service.
- **Timeouts**: Each service send operation has a 10-second timeout.
- **Deduplication**: Duplicate URLs are automatically removed when creating a `Sender`.
