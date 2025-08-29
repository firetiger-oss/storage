package storage_test

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/memory"
)

type prefixFS struct {
	base   fs.FS
	prefix string
}

func (fsys *prefixFS) Open(name string) (fs.File, error) {
	return fsys.base.Open(fsys.prefix + name)
}

// TestFS tests the fs.FS implementation backed by a storage.Bucket.
//
// We only validate a structure with top-level objects because object stores
// don't have a concept of directories, which makes some of the tests impossible
// to pass (e.g., we can't guess the mode of a valid upfront without listing its
// content to determine if it's a leaf object).
func TestFS(t *testing.T) {
	bucket := new(memory.Bucket)
	bucket.PutObject(t.Context(), "test-1", strings.NewReader("ABC"))
	bucket.PutObject(t.Context(), "test-2", strings.NewReader("DE"))
	bucket.PutObject(t.Context(), "test-3", strings.NewReader("FGHIJKL"))

	fsys := storage.FS(t.Context(), storage.SingleBucketRegistry(bucket))

	if err := fstest.TestFS(&prefixFS{base: fsys, prefix: ":memory:"},
		"test-1",
		"test-2",
		"test-3",
	); err != nil {
		t.Error(err)
	}
}

func TestFile(t *testing.T) {
	bucket := new(memory.Bucket)
	bucket.PutObject(t.Context(), "test", strings.NewReader("hello, world!"))

	file := storage.NewFile(context.Background(), bucket, "test", 13)
	if file.Size() != 13 {
		t.Fatalf("unexpected file size: %d != %d", file.Size(), 13)
	}
	if file.Name() != ":memory:test" {
		t.Fatalf("unexpected file name: %q != %q", file.Name(), ":memory:test")
	}

	if b, err := io.ReadAll(io.NewSectionReader(file, 0, 5)); err != nil {
		t.Fatal("unexpected error reading file:", err)
	} else if string(b) != "hello" {
		t.Fatalf("unexpected file data: %q != %q", b, "hello")
	}

	if b, err := io.ReadAll(io.NewSectionReader(file, 7, 5)); err != nil {
		t.Fatal("unexpected error reading file:", err)
	} else if string(b) != "world" {
		t.Fatalf("unexpected file data: %q != %q", b, "world")
	}
}
