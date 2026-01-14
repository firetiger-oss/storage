package notification

import "errors"

var (
	// ErrInvalidEvent is returned when an event cannot be parsed or has invalid data.
	ErrInvalidEvent = errors.New("invalid notification event")

	// ErrHandlerFailed is returned when the target http.Handler returns an error status.
	ErrHandlerFailed = errors.New("handler returned error status")
)
