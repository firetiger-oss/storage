package notification_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/memory"
	"github.com/firetiger-oss/storage/notification"
)

func TestFilterPrefix(t *testing.T) {
	filter := notification.FilterPrefix("sessions/")

	tests := []struct {
		key      string
		expected bool
	}{
		{"sessions/foo/bar.json", true},
		{"sessions/", true},
		{"other/path.txt", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			event := notification.Event{Key: tt.key}
			ok, err := filter(context.Background(), &event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.expected {
				t.Errorf("FilterPrefix(%q) = %v, want %v", tt.key, ok, tt.expected)
			}
		})
	}
}

func TestFilterGlob(t *testing.T) {
	filter := notification.FilterGlob("*.json")

	tests := []struct {
		key      string
		expected bool
	}{
		{"file.json", true},
		{"data.json", true},
		{"file.txt", false},
		{"path/to/file.json", false}, // glob doesn't match path separators
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			event := notification.Event{Key: tt.key}
			ok, err := filter(context.Background(), &event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.expected {
				t.Errorf("FilterGlob(%q) = %v, want %v", tt.key, ok, tt.expected)
			}
		})
	}
}

func TestFilterGlobInvalidPattern(t *testing.T) {
	filter := notification.FilterGlob("[invalid")
	event := notification.Event{Key: "test.txt"}

	_, err := filter(context.Background(), &event)
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

func TestFilterEventType(t *testing.T) {
	filter := notification.FilterEventType(notification.ObjectCreated)

	tests := []struct {
		eventType notification.EventType
		expected  bool
	}{
		{notification.ObjectCreated, true},
		{notification.ObjectDeleted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			event := notification.Event{Type: tt.eventType}
			ok, err := filter(context.Background(), &event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.expected {
				t.Errorf("FilterEventType(%v) = %v, want %v", tt.eventType, ok, tt.expected)
			}
		})
	}
}

func TestFilterEventTypeMultiple(t *testing.T) {
	filter := notification.FilterEventType(notification.ObjectCreated, notification.ObjectDeleted)

	event := notification.Event{Type: notification.ObjectCreated}
	ok, err := filter(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ObjectCreated to pass filter")
	}

	event = notification.Event{Type: notification.ObjectDeleted}
	ok, err = filter(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ObjectDeleted to pass filter")
	}
}

func TestObjectHandlerWithFilterSkipped(t *testing.T) {
	// Track if GetObject is called
	getObjectCalled := false
	bucket := &trackingBucket{
		Bucket: memory.NewBucket(&memory.Entry{
			Key:   "other/file.txt",
			Value: []byte("content"),
		}),
		onGetObject: func() { getObjectCalled = true },
	}

	registry := storage.RegistryFunc(func(ctx context.Context, uri string) (storage.Bucket, error) {
		return bucket, nil
	})

	handlerCalled := false
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Filter that rejects events not starting with "sessions/"
	objectHandler := notification.NewObjectHandler(httpHandler,
		notification.WithRegistry(registry),
		notification.WithFilter(notification.FilterPrefix("sessions/")),
	)

	event := notification.Event{
		Type:   notification.ObjectCreated,
		Bucket: "test-bucket",
		Key:    "other/file.txt", // Does NOT match filter
		Source: "aws:s3",
	}

	err := objectHandler.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}

	if getObjectCalled {
		t.Error("GetObject should not be called when filter rejects event")
	}
	if handlerCalled {
		t.Error("HTTP handler should not be called when filter rejects event")
	}
}

func TestObjectHandlerWithFilterPassed(t *testing.T) {
	bucket := memory.NewBucket()
	_, err := bucket.PutObject(context.Background(), "sessions/data.json",
		strings.NewReader("test data"),
		storage.ContentType("application/json"),
	)
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	registry := storage.RegistryFunc(func(ctx context.Context, uri string) (storage.Bucket, error) {
		return bucket, nil
	})

	handlerCalled := false
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	objectHandler := notification.NewObjectHandler(httpHandler,
		notification.WithRegistry(registry),
		notification.WithFilter(notification.FilterPrefix("sessions/")),
	)

	event := notification.Event{
		Type:   notification.ObjectCreated,
		Bucket: "test-bucket",
		Key:    "sessions/data.json", // Matches filter
		Source: "aws:s3",
	}

	err = objectHandler.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}

	if !handlerCalled {
		t.Error("HTTP handler should be called when filter passes")
	}
}

func TestObjectHandlerMultipleFilters(t *testing.T) {
	bucket := memory.NewBucket()

	registry := storage.RegistryFunc(func(ctx context.Context, uri string) (storage.Bucket, error) {
		return bucket, nil
	})

	handlerCalled := false
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Multiple filters: must be ObjectCreated AND start with "sessions/"
	objectHandler := notification.NewObjectHandler(httpHandler,
		notification.WithRegistry(registry),
		notification.WithFilter(notification.FilterEventType(notification.ObjectCreated)),
		notification.WithFilter(notification.FilterPrefix("sessions/")),
	)

	// Test 1: Fails first filter (wrong event type)
	event := notification.Event{
		Type:   notification.ObjectDeleted,
		Bucket: "test-bucket",
		Key:    "sessions/data.json",
		Source: "aws:s3",
	}
	err := objectHandler.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}
	if handlerCalled {
		t.Error("handler should not be called when first filter rejects")
	}

	// Test 2: Fails second filter (wrong prefix)
	handlerCalled = false
	event = notification.Event{
		Type:   notification.ObjectCreated,
		Bucket: "test-bucket",
		Key:    "other/data.json",
		Source: "aws:s3",
	}
	err = objectHandler.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}
	if handlerCalled {
		t.Error("handler should not be called when second filter rejects")
	}
}

func TestObjectHandlerFilterError(t *testing.T) {
	bucket := memory.NewBucket()

	registry := storage.RegistryFunc(func(ctx context.Context, uri string) (storage.Bucket, error) {
		return bucket, nil
	})

	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	expectedErr := errors.New("filter error")
	errorFilter := func(ctx context.Context, event *notification.Event) (bool, error) {
		return false, expectedErr
	}

	objectHandler := notification.NewObjectHandler(httpHandler,
		notification.WithRegistry(registry),
		notification.WithFilter(errorFilter),
	)

	event := notification.Event{
		Type:   notification.ObjectCreated,
		Bucket: "test-bucket",
		Key:    "data.json",
		Source: "aws:s3",
	}

	err := objectHandler.HandleEvent(context.Background(), &event)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestWithFiltersNoFilters(t *testing.T) {
	handlerCalled := false
	handler := notification.ObjectHandlerFunc(func(ctx context.Context, event *notification.Event) error {
		handlerCalled = true
		return nil
	})

	wrapped := notification.WithFilters(handler)
	event := notification.Event{Type: notification.ObjectCreated, Key: "test.json"}

	err := wrapped.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler should be called when no filters")
	}
}

func TestWithFiltersPassesFilter(t *testing.T) {
	handlerCalled := false
	handler := notification.ObjectHandlerFunc(func(ctx context.Context, event *notification.Event) error {
		handlerCalled = true
		return nil
	})

	wrapped := notification.WithFilters(handler,
		notification.FilterEventType(notification.ObjectCreated),
	)
	event := notification.Event{Type: notification.ObjectCreated, Key: "test.json"}

	err := wrapped.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler should be called when filter passes")
	}
}

func TestWithFiltersRejectsFilter(t *testing.T) {
	handlerCalled := false
	handler := notification.ObjectHandlerFunc(func(ctx context.Context, event *notification.Event) error {
		handlerCalled = true
		return nil
	})

	wrapped := notification.WithFilters(handler,
		notification.FilterEventType(notification.ObjectCreated),
	)
	event := notification.Event{Type: notification.ObjectDeleted, Key: "test.json"}

	err := wrapped.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Error("handler should not be called when filter rejects")
	}
}

func TestWithFiltersPropagatesError(t *testing.T) {
	handler := notification.ObjectHandlerFunc(func(ctx context.Context, event *notification.Event) error {
		return nil
	})

	expectedErr := errors.New("filter error")
	errorFilter := func(ctx context.Context, event *notification.Event) (bool, error) {
		return false, expectedErr
	}

	wrapped := notification.WithFilters(handler, errorFilter)
	event := notification.Event{Type: notification.ObjectCreated, Key: "test.json"}

	err := wrapped.HandleEvent(context.Background(), &event)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestWithFiltersMultipleFilters(t *testing.T) {
	handlerCalled := false
	handler := notification.ObjectHandlerFunc(func(ctx context.Context, event *notification.Event) error {
		handlerCalled = true
		return nil
	})

	wrapped := notification.WithFilters(handler,
		notification.FilterEventType(notification.ObjectCreated),
		notification.FilterPrefix("sessions/"),
	)

	// Test: passes both filters
	event := notification.Event{Type: notification.ObjectCreated, Key: "sessions/data.json"}
	err := wrapped.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler should be called when all filters pass")
	}

	// Test: fails first filter
	handlerCalled = false
	event = notification.Event{Type: notification.ObjectDeleted, Key: "sessions/data.json"}
	err = wrapped.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Error("handler should not be called when first filter rejects")
	}

	// Test: fails second filter
	handlerCalled = false
	event = notification.Event{Type: notification.ObjectCreated, Key: "other/data.json"}
	err = wrapped.HandleEvent(context.Background(), &event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Error("handler should not be called when second filter rejects")
	}
}

// trackingBucket wraps a bucket to track GetObject calls
type trackingBucket struct {
	storage.Bucket
	onGetObject func()
}

func (b *trackingBucket) GetObject(ctx context.Context, key string, options ...storage.GetOption) (io.ReadCloser, storage.ObjectInfo, error) {
	if b.onGetObject != nil {
		b.onGetObject()
	}
	return b.Bucket.GetObject(ctx, key, options...)
}
