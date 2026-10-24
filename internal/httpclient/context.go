package httpclient

import (
	"context"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// BoundContext bounds a send by the service's budget when its context has no
// deadline. A context with a deadline is returned unchanged, so the router's
// send budget and the caller's own deadline always apply as they are.
//
// Parameters:
//   - ctx: the context of the send.
//   - budget: the service's send budget, as it reports through
//     [types.ServiceTimeout], or 0 for [types.DefaultSendTimeout].
//
// Returns:
//   - context.Context: ctx, or ctx bounded by the budget.
//   - context.CancelFunc: releases the bound. The caller always calls it.
func BoundContext(ctx context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}

	if budget <= 0 {
		budget = types.DefaultSendTimeout
	}

	return context.WithTimeout(ctx, budget)
}
