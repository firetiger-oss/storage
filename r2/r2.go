// Package r2 provides a Cloudflare R2 storage backend.
//
// R2 is S3-compatible, so this package wraps the S3 backend with
// R2-specific endpoint configuration.
//
// Usage:
//
//	import _ "github.com/firetiger-oss/storage/r2"
//
//	// Set credentials (R2 uses S3-compatible credentials)
//	// AWS_ACCESS_KEY_ID=<r2-access-key>
//	// AWS_SECRET_ACCESS_KEY=<r2-secret-key>
//	// CF_ACCOUNT_ID=<cloudflare-account-id>
//
//	bucket, err := storage.LoadBucket(ctx, "r2://my-bucket")
package r2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/cache"
	s3pkg "github.com/firetiger-oss/storage/s3"
	"github.com/firetiger-oss/storage/uri"
)

func init() {
	storage.Register("r2", NewRegistry())
}

// ErrMissingAccountID is returned when neither CF_ACCOUNT_ID nor
// CLOUDFLARE_ACCOUNT_ID environment variable is set.
var ErrMissingAccountID = errors.New("r2: CF_ACCOUNT_ID or CLOUDFLARE_ACCOUNT_ID environment variable required")

// NewRegistry creates a storage registry for Cloudflare R2 buckets.
//
// The registry uses environment variables for configuration:
//   - CF_ACCOUNT_ID or CLOUDFLARE_ACCOUNT_ID: Cloudflare account ID (required)
//   - AWS_ACCESS_KEY_ID: R2 API token access key
//   - AWS_SECRET_ACCESS_KEY: R2 API token secret key
//   - AWS_REGION: ignored, R2 uses "auto"
func NewRegistry(options ...func(*s3.Options)) storage.Registry {
	var cachedClient cache.Value[*s3.Client]
	return storage.RegistryFunc(func(ctx context.Context, bucket string) (storage.Bucket, error) {
		accountID := getAccountID()
		if accountID == "" {
			return nil, ErrMissingAccountID
		}

		client, err := cachedClient.Load(func() (*s3.Client, error) {
			cfg, err := config.LoadDefaultConfig(ctx,
				// R2 requires "auto" region, but we use us-east-1 as the SDK alias
				config.WithRegion("auto"),
			)
			if err != nil {
				return nil, err
			}

			endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

			defaultOptions := []func(*s3.Options){
				func(o *s3.Options) {
					o.BaseEndpoint = aws.String(endpoint)
					o.UsePathStyle = true
					// R2 requires us-east-1 for signing
					o.Region = "auto"
				},
			}

			return s3.NewFromConfig(cfg, append(defaultOptions, options...)...), nil
		})
		if err != nil {
			return nil, err
		}

		return &Bucket{
			inner:      s3pkg.NewBucket(client, bucket),
			bucketName: bucket,
		}, nil
	})
}

// getAccountID returns the Cloudflare account ID from environment variables.
func getAccountID() string {
	if id := os.Getenv("CF_ACCOUNT_ID"); id != "" {
		return id
	}
	return os.Getenv("CLOUDFLARE_ACCOUNT_ID")
}

// Bucket wraps an S3 bucket to provide the r2:// URI scheme.
type Bucket struct {
	inner      *s3pkg.Bucket
	bucketName string
}

// Location returns the bucket URI with the r2:// scheme.
func (b *Bucket) Location() string {
	return uri.Join("r2", b.bucketName)
}

// Access verifies that the bucket is accessible.
func (b *Bucket) Access(ctx context.Context) error {
	return b.inner.Access(ctx)
}

// Create creates a new bucket.
func (b *Bucket) Create(ctx context.Context) error {
	return b.inner.Create(ctx)
}

// HeadObject retrieves metadata about an object.
func (b *Bucket) HeadObject(ctx context.Context, key string) (storage.ObjectInfo, error) {
	return b.inner.HeadObject(ctx, key)
}

// GetObject retrieves an object's contents and metadata.
func (b *Bucket) GetObject(ctx context.Context, key string, options ...storage.GetOption) (io.ReadCloser, storage.ObjectInfo, error) {
	return b.inner.GetObject(ctx, key, options...)
}

// PutObject stores an object.
func (b *Bucket) PutObject(ctx context.Context, key string, value io.Reader, options ...storage.PutOption) (storage.ObjectInfo, error) {
	return b.inner.PutObject(ctx, key, value, options...)
}

// DeleteObject removes an object.
func (b *Bucket) DeleteObject(ctx context.Context, key string) error {
	return b.inner.DeleteObject(ctx, key)
}

// DeleteObjects removes multiple objects.
func (b *Bucket) DeleteObjects(ctx context.Context, objects iter.Seq2[string, error]) iter.Seq2[string, error] {
	return b.inner.DeleteObjects(ctx, objects)
}

// CopyObject copies an object within the bucket.
func (b *Bucket) CopyObject(ctx context.Context, from, to string, options ...storage.PutOption) error {
	return b.inner.CopyObject(ctx, from, to, options...)
}

// ListObjects lists objects in the bucket.
func (b *Bucket) ListObjects(ctx context.Context, options ...storage.ListOption) iter.Seq2[storage.Object, error] {
	return b.inner.ListObjects(ctx, options...)
}

// WatchObjects watches for object changes.
func (b *Bucket) WatchObjects(ctx context.Context, options ...storage.ListOption) iter.Seq2[storage.Object, error] {
	return b.inner.WatchObjects(ctx, options...)
}

// PresignGetObject generates a presigned URL for getting an object.
func (b *Bucket) PresignGetObject(ctx context.Context, key string, expiration time.Duration, options ...storage.GetOption) (string, error) {
	return b.inner.PresignGetObject(ctx, key, expiration, options...)
}

// PresignPutObject generates a presigned URL for putting an object.
func (b *Bucket) PresignPutObject(ctx context.Context, key string, expiration time.Duration, options ...storage.PutOption) (string, error) {
	return b.inner.PresignPutObject(ctx, key, expiration, options...)
}

// PresignHeadObject generates a presigned URL for getting object metadata.
func (b *Bucket) PresignHeadObject(ctx context.Context, key string) (string, error) {
	return b.inner.PresignHeadObject(ctx, key)
}

// PresignDeleteObject generates a presigned URL for deleting an object.
func (b *Bucket) PresignDeleteObject(ctx context.Context, key string) (string, error) {
	return b.inner.PresignDeleteObject(ctx, key)
}
