package concurrent

import (
	"context"
)

// DefaultLimit is the default maximum number of concurrent operations
// that are performed by functions that use goroutines to manage concurrency.
const DefaultLimit = 10

type limitKey struct{}

// WithLimit creates a new context with a specified maximum
// concurrency value.
func WithLimit(ctx context.Context, maxConcurrency int) context.Context {
	if maxConcurrency <= 0 {
		return ctx // ignore invalid values, leave the limit as-is
	}
	previousLimit, ok := ctx.Value(limitKey{}).(int)
	if ok && previousLimit < maxConcurrency {
		return ctx // never allow increasing the limit
	}
	return context.WithValue(ctx, limitKey{}, maxConcurrency)
}

// Limit retrieves the maximum concurrency value from the context.
func Limit(ctx context.Context) int {
	maxConcurrency, ok := ctx.Value(limitKey{}).(int)
	if !ok {
		return DefaultLimit
	}
	// hard limits as safeguards
	maxConcurrency = max(maxConcurrency, 1)
	maxConcurrency = min(maxConcurrency, 1000)
	return maxConcurrency
}
