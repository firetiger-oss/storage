package cloudflare

import (
	"github.com/firetiger-oss/tigerblock/notification"
)

func init() {
	notification.DefaultServeOptions = append(notification.DefaultServeOptions,
		notification.WithHandler("POST /cloudflare", NewQueuesHandler),
		notification.WithHandler("POST /cloudflare/batch", NewBatchQueuesHandler),
	)
}
