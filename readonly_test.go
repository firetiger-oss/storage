package storage_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/memory"
)

func TestReadOnlyBucket(t *testing.T) {
	bucket := storage.ReadOnlyBucket(new(memory.Bucket))
	assert := func(err error) {
		if !errors.Is(err, storage.ErrBucketReadOnly) {
			t.Helper()
			t.Fatal("expected ErrBucketReadOnly, got", err)
		}
	}
	ctx := t.Context()
	assert(bucket.Create(ctx))
	assert(bucket.DeleteObject(ctx, "key"))
	assert(bucket.DeleteObjects(ctx, []string{"key1", "key2"}))
	_, err := bucket.PutObject(ctx, "key", strings.NewReader("value"))
	assert(err)
}
