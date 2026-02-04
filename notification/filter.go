package notification

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/firetiger-oss/storage/uri"
)

// Filter examines an event and returns whether to continue processing.
// Returns (true, nil) to continue, (false, nil) to skip, or (_, err) on error.
type Filter func(ctx context.Context, event *Event) (bool, error)

// FilterPrefix returns a filter that accepts events with keys starting with prefix.
func FilterPrefix(prefix string) Filter {
	return func(ctx context.Context, event *Event) (bool, error) {
		_, _, key := uri.Split(event.Object)
		return strings.HasPrefix(key, prefix), nil
	}
}

// FilterGlob returns a filter that accepts events with keys matching the glob pattern.
func FilterGlob(pattern string) Filter {
	return func(ctx context.Context, event *Event) (bool, error) {
		_, _, key := uri.Split(event.Object)
		return path.Match(pattern, key)
	}
}

// FilterEventType returns a filter that accepts only the specified event types.
func FilterEventType(types ...EventType) Filter {
	return func(ctx context.Context, event *Event) (bool, error) {
		return slices.Contains(types, event.Type), nil
	}
}

// NewCreateObjectHandler creates an ObjectHandler that only processes ObjectCreated events.
func NewCreateObjectHandler(fn func(context.Context, *Event) error) ObjectHandler {
	return WithFilters(ObjectHandlerFunc(fn), FilterEventType(ObjectCreated))
}

// NewDeleteObjectHandler creates an ObjectHandler that only processes ObjectDeleted events.
func NewDeleteObjectHandler(fn func(context.Context, *Event) error) ObjectHandler {
	return WithFilters(ObjectHandlerFunc(fn), FilterEventType(ObjectDeleted))
}

// WithFilters wraps an ObjectHandler to apply filters before processing.
// All filters must pass for the event to be handled.
func WithFilters(handler ObjectHandler, filters ...Filter) ObjectHandler {
	if len(filters) == 0 {
		return handler
	}
	return ObjectHandlerFunc(func(ctx context.Context, event *Event) error {
		for _, filter := range filters {
			ok, err := filter(ctx, event)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}
		return handler.HandleEvent(ctx, event)
	})
}
