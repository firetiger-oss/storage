package notification

import (
	"context"
	"time"
)

// EventType represents the type of storage event.
type EventType string

const (
	// ObjectCreated indicates an object was created or updated.
	ObjectCreated EventType = "ObjectCreated"

	// ObjectDeleted indicates an object was deleted.
	ObjectDeleted EventType = "ObjectDeleted"
)

// Event is a unified storage notification event, abstracting differences
// between S3 EventBridge and GCS Pub/Sub notification formats.
type Event struct {
	// Type indicates whether this is a create or delete event.
	Type EventType

	// Bucket is the name of the bucket where the event occurred.
	Bucket string

	// Key is the object key (path) that was affected.
	Key string

	// Size is the object size in bytes (only for create events).
	Size int64

	// ETag is the object's entity tag (only for create events).
	ETag string

	// Time is when the event occurred.
	Time time.Time

	// Source identifies the notification source (e.g., "aws:s3", "gcp:storage").
	Source string

	// Region is the cloud region where the event originated.
	Region string
}

// ObjectHandler processes storage notification events by forwarding them
// as HTTP requests to an underlying http.Handler.
type ObjectHandler interface {
	// HandleEvent processes a storage notification event.
	// For create events, it fetches the object content and forwards it as a POST request.
	// For delete events, it forwards a DELETE request without a body.
	HandleEvent(ctx context.Context, event *Event) error
}

// ObjectHandlerFunc is a function adapter that implements ObjectHandler.
type ObjectHandlerFunc func(context.Context, *Event) error

// HandleEvent implements the ObjectHandler interface.
func (f ObjectHandlerFunc) HandleEvent(ctx context.Context, event *Event) error {
	return f(ctx, event)
}
