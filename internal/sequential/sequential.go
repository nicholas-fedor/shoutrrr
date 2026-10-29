package sequential

import (
	"context"
	"errors"
)

// Send calls send for each item in order and keeps going after an item fails.
// Once ctx ends, it sends no more items and reports ctx's error. That error
// appears once, even when a failed item already returned it.
//
// Parameters:
//   - ctx: the send's context, passed to each call of send.
//   - items: the items to send, such as chats, recipients, or message batches.
//   - send: makes the request for one item. Its error should name the item.
//
// Returns:
//   - error: the errors of the items that failed and ctx's error when it ended
//     the send, joined with [errors.Join], or nil when every item was sent.
func Send[T any](ctx context.Context, items []T, send func(context.Context, T) error) error {
	var errs []error

	for _, item := range items {
		if err := ctx.Err(); err != nil {
			if !errors.Is(errors.Join(errs...), err) {
				errs = append(errs, err)
			}

			break
		}

		if err := send(ctx, item); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
