package memory_test

import (
	"testing"

	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/memory"
	storagetest "github.com/firetiger-oss/storage/test"
)

func TestMemoryStorage(t *testing.T) {
	storagetest.TestStorage(t, func(*testing.T) (storage.Bucket, error) {
		return new(memory.Bucket), nil
	})
}
