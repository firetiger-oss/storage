package cloudflare

import (
	"net/http"

	"github.com/firetiger-oss/storage/notification"
)

func init() {
	notification.DefaultServeOptions = append(notification.DefaultServeOptions,
		notification.WithHandler("POST /cloudflare", NewQueuesHandler),
	)

	notification.DefaultBatchServeOptions = append(notification.DefaultBatchServeOptions,
		notification.WithBatchHandler("POST /cloudflare/batch",
			func(h notification.BatchObjectHandler) http.Handler {
				return NewBatchQueuesEventBatchHandler(h)
			},
		),
	)
}
