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

- **Functions**:
  - `shoutrrr.Send(url string, message string) error`
  - `shoutrrr.SendContext(ctx context.Context, url string, message string, params *types.Params) error`
- **Behavior**: Initializes a service from the provided URL, sends the message within the service's send budget, closes
  the service, and returns any error. A send failure is a `*types.TargetError`, which names the service but never
  includes the URL, so it is safe to log.
- **Parameters**: `Send` takes no parameters. Use `SendContext` to pass parameters such as a title, and to cancel the
  send or bound it with a deadline. `params` may be `nil`.

!!! Example
    ```go title="Send to a Single Slack URL"
    url := "slack://token-a/token-b/token-c"
    err := shoutrrr.Send(url, "Hello, Slack!")
    if err != nil {
        fmt.Println("Error:", err)
    }
    ```

<!-- markdownlint-disable -->
!!! Example
    ```go title="Send with a Title and a Deadline"
    import (
        "context"
        "fmt"
        "time"

        "github.com/nicholas-fedor/shoutrrr"
        "github.com/nicholas-fedor/shoutrrr/pkg/types"
    )

    func notify(url string) error {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        params := types.Params{}
        params.SetTitle("Alert")

        if err := shoutrrr.SendContext(ctx, url, "System alert!", &params); err != nil {
            return fmt.Errorf("sending alert: %w", err)
        }

        return nil
    }
    ```
<!-- markdownlint-restore -->

### Sender

Creates a `Sender` (`*ServiceRouter`) to manage multiple service URLs, support message queuing, and allow parameter customization.

- **Function**: `shoutrrr.CreateSenderWithOptions(opts types.SenderOptions, urls ...string) (*ServiceRouter, error)`.
  `shoutrrr.NewSenderWithOptions` also takes a logger. `CreateSender` and `NewSender` are deprecated.
- **Options**: `types.SenderOptions{}` uses the defaults. `HTTPClient` is injected into HTTP services. `DialContext` is injected into TCP services that implement `types.DialContextSetter`. `shoutrrr.Send` and `shoutrrr.SendContext` cannot take these options.
- **Methods**:
  - `Send(message string, params *types.Params) []error`: Sends a message to all configured services.
  - `SendContext(ctx context.Context, message string, params *types.Params) []error`: Like `Send`, and returns
    when `ctx` is canceled or its deadline passes.
  - `SendItems(items []types.MessageItem, params types.Params) []error`: Sends structured message items to services that support rich formatting.
  - `SendItemsContext(ctx context.Context, items []types.MessageItem, params types.Params) []error`: Like
    `SendItems`, bounded by `ctx`.
  - `SendAsync(message string, params *types.Params) chan error`: Sends a message asynchronously and returns a channel of errors.
  - `Enqueue(message string, v ...any)`: Queues a formatted message for later sending.
  - `Flush(params *types.Params)`: Sends all queued messages and resets the queue.
  - `Close() error`: Closes services that hold resources between sends, such as MQTT connections.
- **Behavior**: Initializes a service for each URL and sends to all of them concurrently. `Send`, `SendContext`,
  `SendItems`, and `SendItemsContext` return one entry per configured URL, in the same order, and the entry is `nil`
  when that send succeeded.

<!-- markdownlint-disable -->
!!! Example
    ```go title="Create Sender with Multiple URLs"
    urls := []string{
        "slack://token-a/token-b/token-c",
        "telegram://110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw@telegram?channels=@mychannel",
    }
    sender, err := shoutrrr.CreateSenderWithOptions(types.SenderOptions{}, urls...)
    if err != nil {
        log.Fatal(err)
    }
    defer sender.Close()

    params := types.Params{}
    params.SetTitle("Test Notification")
    for i, err := range sender.Send("Hello, world!", &params) {
        if err != nil {
            fmt.Printf("Error sending to URL %d: %v\n", i, err)
        }
    }
    ```
<!-- markdownlint-restore -->

### Message Queuing

Allows queuing messages for deferred sending, useful for aggregating notifications during a process.

- Queues messages with `Enqueue` and sends them with `Flush`. Queued messages use the `params` provided during `Flush`.

<!-- markdownlint-disable -->
!!! Example
    ```go title="Queue and Flush Notifications"
    url := "discord://abc123@123456789"
    sender, err := shoutrrr.CreateSenderWithOptions(types.SenderOptions{}, url)
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
    sender, err := shoutrrr.CreateSenderWithOptions(types.SenderOptions{}, "discord://token@webhookid")
    if err != nil {
        log.Fatal(err)
    }
    errs := sender.SendItems(items, types.Params{})
    ```

### Context Propagation

`shoutrrr.SendContext`, `ServiceRouter.SendContext`, and `ServiceRouter.SendItemsContext` take a caller context.
Services that implement `types.ContextSender` or `types.ContextAttachmentSender` receive a context derived from it with
the service's send budget, so cancellation and deadlines stop their requests. For other services, the call returns when
the context ends, and the service's error reports the context's error. `Send` and `SendItems` use a background context.

### Per-Target Errors

`*ServiceRouter.Send`, `SendContext`, `SendItems`, and `SendItemsContext` return one entry per configured URL, in the
order the URLs were given, and the entry is `nil` when that send succeeded. `SendAsync` reports the errors in the order
the sends finish. `Route` and `shoutrrr.SendContext` return a single error. Each failure is a `*types.TargetError`, whose
`URL` field holds the service ID (such as `discord`) and whose `Index` field holds the URL's position. It never contains
the service URL, so it is safe to log, and it supports `errors.Unwrap`, `errors.Is`, and `errors.As`.

!!! Example
    ```go title="Handle Per-Target Errors"
    errs := sender.Send("deploy complete", nil)
    for i, err := range errs {
        if err == nil {
            continue
        }
        var targetErr *types.TargetError
        if errors.As(err, &targetErr) {
            log.Printf("failed to send to URL %d (%s): %v", i, targetErr.URL, targetErr.Err)
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
    sender, err := shoutrrr.CreateSenderWithOptions(types.SenderOptions{}, "discord://token@webhookid")
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
    params := types.Params{}
    params.SetTitle("Alert")
    if err := shoutrrr.SendContext(context.Background(), url, "System alert!", &params); err != nil {
        fmt.Println("Error:", err)
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
    sender, err := shoutrrr.CreateSenderWithOptions(types.SenderOptions{}, urls...)
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

- **Error Handling**: `shoutrrr.Send` and `shoutrrr.SendContext` return a single error. `Sender.Send`, `SendContext`,
  `SendItems`, and `SendItemsContext` return one entry per URL, `nil` on success, so check each entry rather than the
  length of the slice. Each failure is a `*types.TargetError` naming the service, never its URL.
- **Parameters**: `params` is a `*types.Params` value for `Send`, `SendContext`, `SendAsync`, and `Flush`, including
  `shoutrrr.SendContext`. `SendItems` accepts `types.Params` by value. Use setter methods such as `SetTitle`, `SetMessage`, and `SetLevel` to configure service-specific options. Use `shoutrrr docs` to view supported parameters for each service.
- **Timeouts**: The default is 10 seconds per service. A longer service timeout extends that service. A positive `SenderOptions.Timeout` is the exact fixed timeout for every service.
- **Duplicate URLs**: A `Sender` sends to every URL it is given, including duplicates. The CLI's `send` command removes
  duplicate URLs before sending.
