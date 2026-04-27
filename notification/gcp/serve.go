package gcp

import (
	"os"

	"github.com/firetiger-oss/tigerblock/notification"
)

func init() {
	// GCP Cloud Run sets PORT environment variable
	if port := os.Getenv("PORT"); port != "" {
		notification.DefaultServeOptions = append(notification.DefaultServeOptions,
			notification.WithPort(port),
		)
	}

	// Register Pub/Sub HTTP handler
	notification.DefaultServeOptions = append(notification.DefaultServeOptions,
		notification.WithHandler("POST /gcp", NewPubSubHandler),
	)
}
