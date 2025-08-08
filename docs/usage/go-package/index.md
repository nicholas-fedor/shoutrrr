# Using the Shoutrrr Package

## Overview

The Shoutrrr Go package (`github.com/nicholas-fedor/shoutrrr`) enables sending notifications to various services (e.g., `discord`, `slack`, `telegram`, `smtp`, etc.) using service URLs. It provides two primary methods: a direct `Send` function for simple use cases and a `Sender` struct for advanced scenarios with multiple URLs, message queuing, and parameter customization.

## Usage

```go title="Go Import Statement"
import "github.com/nicholas-fedor/shoutrrr"
```

### Direct Send

Sends a notification to a single service URL with an optional parameters map.

- **Function**: `shoutrrr.Send(url string, message string, params ...map[string]string) []error`
- **Behavior**: Initializes a service from the provided URL, sends the message, and returns any errors. If a `params` map is provided, it customizes the notification (e.g., setting a title).

!!! Example
    ```go title="Send to a Single Slack URL"
    url := "slack://token-a/token-b/token-c"
    errs := shoutrrr.Send(url, "Hello, Slack!")
    if len(errs) > 0 {
        // Handle errors
        for _, err := range errs {
            fmt.Println("Error:", err)
        }
    }
    ```

### Sender

Creates a `Sender` (`*ServiceRouter`) to manage multiple service URLs, support message queuing, and allow parameter customization.

- **Function**: `shoutrrr.CreateSender(urls ...string) (*ServiceRouter, error)`
- **Methods**:
  - `Send(message string, params map[string]string) []error`: Sends a message to all configured services.
  - `Enqueue(message string, a ...interface{})`: Queues a formatted message for later sending.
  - `Flush(params map[string]string) []error`: Sends all queued messages and resets the queue.
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
    params := map[string]string{"title": "Test Notification"}
    errs := sender.Send("Hello, world!", params)
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
    defer sender.Flush(map[string]string{"title": "Work Result"})

    sender.Enqueue("Started doing work")
    if err := doWork(); err != nil {
        sender.Enqueue("Error: %v", err)
        return
    }
    sender.Enqueue("Work completed successfully!")
    ```
<!-- markdownlint-restore -->

## Examples

<!-- markdownlint-disable -->
### Send with Parameters and Error Handling

!!! Example
    ```go title="Send with Title and Error Handling"
    url := "discord://abc123@123456789"
    params := map[string]string{"title": "Alert"}
    errs := shoutrrr.Send(url, "System alert!", params)
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
    defer sender.Flush(map[string]string{"title": "Task Summary"})
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

- **Error Handling**: Both `Send` and `Sender.Send` return a slice of errors, one per service. Check `len(errs) > 0` to handle failures.
- **Parameters**: The `params` map supports service-specific options (e.g., `title` for Discord, Slack). Use `shoutrrr docs` to view supported parameters for each service.
- **Timeouts**: Each service send operation has a 10-second timeout.
- **Deduplication**: Duplicate URLs are automatically removed when creating a `Sender`.
