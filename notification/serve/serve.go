// Package serve provides a unified notification handler that auto-detects
// the runtime environment and serves appropriate endpoints for AWS, GCP,
// and Cloudflare storage notification events.
package serve

import (
	"cmp"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/firetiger-oss/storage/notification"
	"github.com/firetiger-oss/storage/notification/aws"
	"github.com/firetiger-oss/storage/notification/cloudflare"
	"github.com/firetiger-oss/storage/notification/gcp"
)

// Serve starts a notification handler that auto-detects the runtime environment.
//
// In AWS Lambda: Uses S3LambdaHandler for direct Lambda invocation
// In HTTP mode: Serves AWS EventBridge, GCP Pub/Sub, and Cloudflare Queues endpoints
//
// Environment detection:
//   - lambdacontext.FunctionName set -> Lambda mode
//   - Otherwise -> HTTP mode on PORT (default 8080)
func Serve(handler notification.ObjectHandler, options ...Option) error {
	opts := defaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	// AWS Lambda mode - use lambdacontext.FunctionName which is populated by the runtime
	if lambdacontext.FunctionName != "" {
		lambdaHandler := aws.NewS3LambdaHandler(handler)
		lambda.Start(lambdaHandler.HandleEvent)
		return nil
	}

	// HTTP mode
	mux := http.NewServeMux()
	mux.Handle("POST "+opts.awsPath, aws.NewS3EventBridgeHandler(handler))
	mux.Handle("POST "+opts.gcpPath, gcp.NewPubSubHandler(handler))
	mux.Handle("POST "+opts.cloudflarePath, cloudflare.NewQueuesHandler(handler))

	if opts.healthPath != "" {
		mux.HandleFunc("GET "+opts.healthPath, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	return http.ListenAndServe(":"+opts.port, mux)
}

type options struct {
	awsPath        string
	gcpPath        string
	cloudflarePath string
	healthPath     string
	port           string
}

func defaultOptions() options {
	return options{
		awsPath:        "/aws",
		gcpPath:        "/gcp",
		cloudflarePath: "/cloudflare",
		healthPath:     "/health",
		port:           cmp.Or(os.Getenv("PORT"), "8080"),
	}
}

// Option is a functional option for configuring Serve.
type Option func(*options)

// WithAWSPath sets the HTTP path for AWS EventBridge notifications (default: "/aws")
func WithAWSPath(path string) Option {
	return func(o *options) { o.awsPath = path }
}

// WithGCPPath sets the HTTP path for GCP Pub/Sub notifications (default: "/gcp")
func WithGCPPath(path string) Option {
	return func(o *options) { o.gcpPath = path }
}

// WithCloudflarePath sets the HTTP path for Cloudflare Queues notifications (default: "/cloudflare")
func WithCloudflarePath(path string) Option {
	return func(o *options) { o.cloudflarePath = path }
}

// WithHealthPath sets the health check endpoint path (default: "/health", empty to disable)
func WithHealthPath(path string) Option {
	return func(o *options) { o.healthPath = path }
}

// WithPort sets the HTTP port (default: PORT env or "8080")
func WithPort(port string) Option {
	return func(o *options) { o.port = port }
}

// ServeBatch starts a batch notification handler that auto-detects the runtime environment.
//
// In AWS Lambda: Uses S3LambdaBatchHandler for direct Lambda invocation
// In HTTP mode: Serves Cloudflare Queues batch endpoint
//
// Environment detection:
//   - lambdacontext.FunctionName set -> Lambda mode
//   - Otherwise -> HTTP mode on PORT (default 8080)
func ServeBatch(handler notification.BatchObjectHandler, options ...BatchOption) error {
	opts := defaultBatchOptions()
	for _, opt := range options {
		opt(&opts)
	}

	// AWS Lambda mode
	if lambdacontext.FunctionName != "" {
		lambdaHandler := aws.NewS3LambdaBatchHandler(handler)
		lambda.Start(lambdaHandler.HandleEvent)
		return nil
	}

	// HTTP mode
	mux := http.NewServeMux()
	mux.Handle("POST "+opts.cloudflareBatchPath, cloudflare.NewBatchQueuesEventBatchHandler(handler))

	if opts.healthPath != "" {
		mux.HandleFunc("GET "+opts.healthPath, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	return http.ListenAndServe(":"+opts.port, mux)
}

type batchOptions struct {
	cloudflareBatchPath string
	healthPath          string
	port                string
}

func defaultBatchOptions() batchOptions {
	return batchOptions{
		cloudflareBatchPath: "/cloudflare/batch",
		healthPath:          "/health",
		port:                cmp.Or(os.Getenv("PORT"), "8080"),
	}
}

// BatchOption is a functional option for configuring ServeBatch.
type BatchOption func(*batchOptions)

// WithBatchCloudflarePath sets the HTTP path for Cloudflare Queues batch notifications (default: "/cloudflare/batch")
func WithBatchCloudflarePath(path string) BatchOption {
	return func(o *batchOptions) { o.cloudflareBatchPath = path }
}

// WithBatchHealthPath sets the health check endpoint path for ServeBatch (default: "/health", empty to disable)
func WithBatchHealthPath(path string) BatchOption {
	return func(o *batchOptions) { o.healthPath = path }
}

// WithBatchPort sets the HTTP port for ServeBatch (default: PORT env or "8080")
func WithBatchPort(port string) BatchOption {
	return func(o *batchOptions) { o.port = port }
}
