package sequential

import (
	"context"
	"errors"
	"time"
)

// Policy decides how long each item of a send may take. Use [SharedDeadline]
// or [PerTarget]. The zero value is [SharedDeadline].
type Policy struct {
	// perTarget is the time one target is budgeted, or zero for items that
	// share the send's deadline.
	perTarget time.Duration
}

// SharedDeadline gives every item the send's deadline. Use it for items that
// are parts of one exchange, such as the batches of one message or the stanzas
// of one session, where cutting one item short would harm the others.
var SharedDeadline = Policy{perTarget: 0}

// PerTarget gives each independent target, such as a chat, recipient, or
// room, the longer of perTarget and an equal share of the time left before the
// send's deadline, but never past that deadline. A target that does not answer
// then cannot use up the time of the others. Time a target does not use
// carries over to the targets after it, and the last target has all the time
// left.
//
// Under a service's own budget of perTarget for each target, every target gets
// perTarget. A caller's longer deadline makes the shares longer. A deadline too
// short for every target leaves each target at least perTarget, so targets are
// sent one after another until the time runs out.
//
// Parameters:
//   - perTarget: the time one target's requests are budgeted. A value that is
//     not positive gives [SharedDeadline].
//
// Returns:
//   - Policy: the policy.
func PerTarget(perTarget time.Duration) Policy {
	return Policy{perTarget: perTarget}
}

// Send calls send for each item in order and keeps going after an item fails.
// Each item gets the context that policy gives it.
//
// Once ctx ends, Send sends no more items and reports ctx's error. That error
// appears once, even when a failed item already returned it.
//
// Parameters:
//   - ctx: the send's context.
//   - items: the items to send, such as chats, recipients, or message batches.
//   - policy: decides how long each item may take.
//   - send: makes the requests for one item. Its error should name the item.
//
// Returns:
//   - error: the errors of the items that failed and ctx's error when it ended
//     the send, joined with [errors.Join], or nil when every item was sent.
func Send[T any](
	ctx context.Context,
	items []T,
	policy Policy,
	send func(context.Context, T) error,
) error {
	var errs []error

	// reported is set when the item that saw ctx end returned ctx's error. An
	// item whose own share ran out returns the same sentinel while ctx is
	// still live, so the check looks at ctx after each item rather than at
	// the errors collected so far.
	reported := false

	for i, item := range items {
		if err := ctx.Err(); err != nil {
			if !reported {
				errs = append(errs, err)
			}

			break
		}

		itemCtx, cancel := policy.itemContext(ctx, len(items)-i)
		err := send(itemCtx, item)

		cancel()

		if err != nil {
			errs = append(errs, err)
			reported = ctx.Err() != nil && errors.Is(err, ctx.Err())
		}
	}

	return errors.Join(errs...)
}

// itemContext returns the context for one item.
//
// Parameters:
//   - ctx: the send's context.
//   - left: the number of items left, this one included.
//
// Returns:
//   - context.Context: ctx bounded by the item's share of the time, or ctx as
//     it is when the policy shares no time, ctx has no deadline, or the share
//     reaches ctx's deadline.
//   - context.CancelFunc: releases the item's context.
func (p Policy) itemContext(ctx context.Context, left int) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if p.perTarget <= 0 || !ok {
		return ctx, func() {}
	}

	remaining := time.Until(deadline)

	share := max(remaining/time.Duration(left), p.perTarget)
	if share >= remaining {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, share)
}
