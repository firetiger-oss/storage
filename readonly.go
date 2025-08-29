package storage

import (
	"context"
	"io"
	"iter"
)

func ReadOnlyBucket(bucket Bucket) Bucket {
	return &readOnlyBucket{bucket: bucket}
}

type readOnlyBucket struct {
	bucket Bucket
}

func (b *readOnlyBucket) Location() string {
	return b.bucket.Location()
}

func (b *readOnlyBucket) Access(ctx context.Context) error {
	return b.bucket.Access(ctx)
}

func (b *readOnlyBucket) Create(ctx context.Context) error {
	return ErrBucketReadOnly
}

func (b *readOnlyBucket) HeadObject(ctx context.Context, key string) (ObjectInfo, error) {
	return b.bucket.HeadObject(ctx, key)
}

func (b *readOnlyBucket) GetObject(ctx context.Context, key string, options ...GetOption) (io.ReadCloser, ObjectInfo, error) {
	return b.bucket.GetObject(ctx, key, options...)
}

func (b *readOnlyBucket) PutObject(ctx context.Context, key string, value io.Reader, options ...PutOption) (ObjectInfo, error) {
	return ObjectInfo{}, ErrBucketReadOnly
}

func (b *readOnlyBucket) DeleteObject(ctx context.Context, key string) error {
	return ErrBucketReadOnly
}

func (b *readOnlyBucket) DeleteObjects(ctx context.Context, keys []string) error {
	return ErrBucketReadOnly
}

func (b *readOnlyBucket) ListObjects(ctx context.Context, options ...ListOption) iter.Seq2[Object, error] {
	return b.bucket.ListObjects(ctx, options...)
}

func (b *readOnlyBucket) WatchObjects(ctx context.Context, options ...ListOption) iter.Seq2[Object, error] {
	return b.bucket.WatchObjects(ctx, options...)
}

func (b *readOnlyBucket) PresignGetObject(ctx context.Context, key string, options ...GetOption) (string, error) {
	return b.bucket.PresignGetObject(ctx, key, options...)
}

func (b *readOnlyBucket) PresignPutObject(ctx context.Context, key string, options ...PutOption) (string, error) {
	return "", ErrBucketReadOnly
}

func (b *readOnlyBucket) PresignHeadObject(ctx context.Context, key string) (string, error) {
	return b.bucket.PresignHeadObject(ctx, key)
}

func (b *readOnlyBucket) PresignDeleteObject(ctx context.Context, key string) (string, error) {
	return "", ErrBucketReadOnly
}
