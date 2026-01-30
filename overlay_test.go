package storage_test

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/internal/sequtil"
	"github.com/firetiger-oss/storage/memory"
)

func TestOverlay(t *testing.T) {
	t.Run("Location returns write layer location", func(t *testing.T) {
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket()
		bucket := storage.Overlay(writeLayer, readLayer)
		if bucket.Location() != writeLayer.Location() {
			t.Errorf("expected %s, got %s", writeLayer.Location(), bucket.Location())
		}
	})

	t.Run("HeadObject returns from write layer when present", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("write")})
		readLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("read-layer")})
		bucket := storage.Overlay(writeLayer, readLayer)

		info, err := bucket.HeadObject(ctx, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Size != 5 {
			t.Errorf("expected size 5, got %d", info.Size)
		}
	})

	t.Run("HeadObject falls back to read layer when not in write layer", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("read-layer")})
		bucket := storage.Overlay(writeLayer, readLayer)

		info, err := bucket.HeadObject(ctx, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Size != 10 {
			t.Errorf("expected size 10, got %d", info.Size)
		}
	})

	t.Run("HeadObject returns error when not in either layer", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket()
		bucket := storage.Overlay(writeLayer, readLayer)

		_, err := bucket.HeadObject(ctx, "missing.txt")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetObject returns from write layer when present", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("write")})
		readLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("read-layer")})
		bucket := storage.Overlay(writeLayer, readLayer)

		reader, _, err := bucket.GetObject(ctx, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		data, _ := io.ReadAll(reader)
		if string(data) != "write" {
			t.Errorf("expected 'write', got %q", string(data))
		}
	})

	t.Run("GetObject falls back to read layer when not in write layer", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("read-layer")})
		bucket := storage.Overlay(writeLayer, readLayer)

		reader, _, err := bucket.GetObject(ctx, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		data, _ := io.ReadAll(reader)
		if string(data) != "read-layer" {
			t.Errorf("expected 'read-layer', got %q", string(data))
		}
	})

	t.Run("PutObject writes to write layer only", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket()
		bucket := storage.Overlay(writeLayer, readLayer)

		_, err := bucket.PutObject(ctx, "test.txt", strings.NewReader("content"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := writeLayer.HeadObject(ctx, "test.txt"); err != nil {
			t.Error("expected object in write layer")
		}
		if _, err := readLayer.HeadObject(ctx, "test.txt"); err == nil {
			t.Error("did not expect object in read layer")
		}
	})

	t.Run("DeleteObject deletes from write layer only", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("write")})
		readLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("read-layer")})
		bucket := storage.Overlay(writeLayer, readLayer)

		if err := bucket.DeleteObject(ctx, "test.txt"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := writeLayer.HeadObject(ctx, "test.txt"); err == nil {
			t.Error("expected object deleted from write layer")
		}
		if _, err := readLayer.HeadObject(ctx, "test.txt"); err != nil {
			t.Error("expected object still in read layer")
		}
	})

	t.Run("ListObjects merges in sorted order with deduplication", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket(
			&memory.Entry{Key: "a.txt", Value: []byte("a-write")},
			&memory.Entry{Key: "c.txt", Value: []byte("c-write")},
		)
		readLayer := memory.NewBucket(
			&memory.Entry{Key: "a.txt", Value: []byte("a-read")},
			&memory.Entry{Key: "b.txt", Value: []byte("b-read")},
		)
		bucket := storage.Overlay(writeLayer, readLayer)

		objects, err := sequtil.Collect(bucket.ListObjects(ctx))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		keys := make([]string, len(objects))
		for i, obj := range objects {
			keys[i] = obj.Key
		}

		expected := []string{"a.txt", "b.txt", "c.txt"}
		if !slices.Equal(keys, expected) {
			t.Errorf("expected sorted keys %v, got %v", expected, keys)
		}
	})

	t.Run("Access checks both layers", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket()
		bucket := storage.Overlay(writeLayer, readLayer)

		if err := bucket.Access(ctx); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Create only creates write layer", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket()
		bucket := storage.Overlay(writeLayer, readLayer)

		err := bucket.Create(ctx)
		if err != storage.ErrBucketExist {
			t.Errorf("expected ErrBucketExist, got %v", err)
		}
	})

	t.Run("WithOverlay adapter", func(t *testing.T) {
		ctx := t.Context()
		writeLayer := memory.NewBucket()
		readLayer := memory.NewBucket(&memory.Entry{Key: "test.txt", Value: []byte("read-layer")})

		bucket := storage.AdaptBucket(writeLayer, storage.WithOverlay(readLayer))

		reader, _, err := bucket.GetObject(ctx, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		data, _ := io.ReadAll(reader)
		if string(data) != "read-layer" {
			t.Errorf("expected 'read-layer', got %q", string(data))
		}
	})
}
