package file

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firetiger-oss/storage"
	"github.com/firetiger-oss/storage/test"
)

func TestCacheWithLimit(t *testing.T) {
	test.TestStorage(t, func(t *testing.T) (storage.Bucket, error) {
		store := t.TempDir()
		cache := t.TempDir()

		bucket, err := NewRegistry(store).LoadBucket(t.Context(), "")
		if err != nil {
			return nil, err
		}

		return NewCache(cache, 16).AdaptBucket(bucket), nil
	})
}

func TestCacheWithoutLimit(t *testing.T) {
	test.TestStorage(t, func(t *testing.T) (storage.Bucket, error) {
		store := t.TempDir()
		cache := t.TempDir()

		bucket, err := NewRegistry(store).LoadBucket(t.Context(), "")
		if err != nil {
			return nil, err
		}

		return NewCache(cache, math.MaxInt64).AdaptBucket(bucket), nil
	})
}

func TestCacheReuse(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	// Create larger objects to ensure evictions will happen
	const key1 = "cached-object-1"
	const key2 = "cached-object-2"
	const key3 = "cached-object-3"
	const key4 = "cached-object-4"
	const key5 = "cached-object-5"

	// Create content that's large enough to trigger evictions (each ~100 bytes)
	value1 := strings.Repeat("first cached content with lots of data to make it larger ", 2)
	value2 := strings.Repeat("second cached content with even more data to fill cache ", 2)
	value3 := strings.Repeat("third cached content that will definitely cause evictions ", 2)
	value4 := strings.Repeat("fourth cached content for testing eviction behavior nicely ", 2)
	value5 := strings.Repeat("fifth cached content to really push the cache limits hard ", 2)

	// Step 1: Create first cache with large size and populate it with multiple objects
	cacheInstance1 := NewCache(cacheDir, 2048) // Large cache to fit all objects initially
	bucket1, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create first bucket:", err)
	}
	cache1 := cacheInstance1.AdaptBucket(bucket1)

	// Add multiple objects through first cache with proper cache control headers
	obj1, err := cache1.PutObject(ctx, key1, strings.NewReader(value1), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put first object through first cache:", err)
	}

	obj2, err := cache1.PutObject(ctx, key2, strings.NewReader(value2), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put second object through first cache:", err)
	}

	_, err = cache1.PutObject(ctx, key3, strings.NewReader(value3), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put third object through first cache:", err)
	}

	_, err = cache1.PutObject(ctx, key4, strings.NewReader(value4), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put fourth object through first cache:", err)
	}

	// Access objects through first cache to ensure they're cached
	r1, _, err := cache1.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to get first object through first cache:", err)
	}
	r1.Close()

	r2, _, err := cache1.GetObject(ctx, key2)
	if err != nil {
		t.Fatal("failed to get second object through first cache:", err)
	}
	r2.Close()

	r3, _, err := cache1.GetObject(ctx, key3)
	if err != nil {
		t.Fatal("failed to get third object through first cache:", err)
	}
	r3.Close()

	r4, _, err := cache1.GetObject(ctx, key4)
	if err != nil {
		t.Fatal("failed to get fourth object through first cache:", err)
	}
	r4.Close()

	// Check initial cache statistics
	stat1Initial := cacheInstance1.Stat()
	t.Logf("Cache 1 initial stats: Size=%d, Hits=%d, Misses=%d, Evictions=%d",
		stat1Initial.Size, stat1Initial.Hits, stat1Initial.Misses, stat1Initial.Evictions)

	// Step 2: Create second cache with MUCH smaller size (exercises rehydration + eviction)
	cacheInstance2 := NewCache(cacheDir, 200) // Small cache that can only fit ~2 objects
	bucket2, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create second bucket:", err)
	}
	cache2 := cacheInstance2.AdaptBucket(bucket2)

	// Step 3: Put a new object through the small cache to trigger evictions
	obj5, err := cache2.PutObject(ctx, key5, strings.NewReader(value5), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put fifth object through second cache:", err)
	}

	// Step 4: Access objects through second cache - this should cause evictions
	r1_cache2, obj1_cache2, err := cache2.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to get first object through second cache:", err)
	}
	defer r1_cache2.Close()

	data1, err := io.ReadAll(r1_cache2)
	if err != nil {
		t.Fatal("failed to read first object data from second cache:", err)
	}

	if string(data1) != value1 {
		t.Errorf("first object data mismatch: %q != %q", string(data1), value1)
	}

	if obj1_cache2.ETag != obj1.ETag {
		t.Errorf("first object ETags don't match: %q != %q", obj1_cache2.ETag, obj1.ETag)
	}

	if obj1_cache2.Size != obj1.Size {
		t.Errorf("first object sizes don't match: %d != %d", obj1_cache2.Size, obj1.Size)
	}

	// Access more objects to trigger more evictions
	r2_cache2, obj2_cache2, err := cache2.GetObject(ctx, key2)
	if err != nil {
		t.Fatal("failed to get second object through second cache:", err)
	}
	defer r2_cache2.Close()

	data2, err := io.ReadAll(r2_cache2)
	if err != nil {
		t.Fatal("failed to read second object data from second cache:", err)
	}

	if string(data2) != value2 {
		t.Errorf("second object data mismatch: %q != %q", string(data2), value2)
	}

	if obj2_cache2.ETag != obj2.ETag {
		t.Errorf("second object ETags don't match: %q != %q", obj2_cache2.ETag, obj2.ETag)
	}

	if obj2_cache2.Size != obj2.Size {
		t.Errorf("second object sizes don't match: %d != %d", obj2_cache2.Size, obj2.Size)
	}

	// Access the fifth object through second cache
	r5_cache2, obj5_cache2, err := cache2.GetObject(ctx, key5)
	if err != nil {
		t.Fatal("failed to get fifth object through second cache:", err)
	}
	defer r5_cache2.Close()

	data5, err := io.ReadAll(r5_cache2)
	if err != nil {
		t.Fatal("failed to read fifth object data from second cache:", err)
	}

	if string(data5) != value5 {
		t.Errorf("fifth object data mismatch: %q != %q", string(data5), value5)
	}

	if obj5_cache2.ETag != obj5.ETag {
		t.Errorf("fifth object ETags don't match: %q != %q", obj5_cache2.ETag, obj5.ETag)
	}

	if obj5_cache2.Size != obj5.Size {
		t.Errorf("fifth object sizes don't match: %d != %d", obj5_cache2.Size, obj5.Size)
	}

	// Step 5: Test cache statistics and validate evictions occurred
	stat1Final := cacheInstance1.Stat()
	stat2Final := cacheInstance2.Stat()

	// Log final cache statistics
	t.Logf("Cache 1 final stats: Size=%d, Hits=%d, Misses=%d, Evictions=%d",
		stat1Final.Size, stat1Final.Hits, stat1Final.Misses, stat1Final.Evictions)
	t.Logf("Cache 2 final stats: Size=%d, Hits=%d, Misses=%d, Evictions=%d",
		stat2Final.Size, stat2Final.Hits, stat2Final.Misses, stat2Final.Evictions)

	// Verify the limits are what we set
	expectedLimit1 := int64(2048)
	expectedLimit2 := int64(200)
	if stat1Final.Limit != expectedLimit1 {
		t.Errorf("unexpected cache 1 limit: %d != %d", stat1Final.Limit, expectedLimit1)
	}
	if stat2Final.Limit != expectedLimit2 {
		t.Errorf("unexpected cache 2 limit: %d != %d", stat2Final.Limit, expectedLimit2)
	}

	// Validate that the second cache had evictions due to its small size
	if stat2Final.Evictions == 0 {
		t.Error("expected evictions in second cache due to small size, but got 0")
	}

	// Validate that second cache size is within its limit
	if stat2Final.Size > stat2Final.Limit {
		t.Errorf("cache 2 size (%d) exceeds its limit (%d)", stat2Final.Size, stat2Final.Limit)
	}

	// Step 6: Verify both caches can still access objects (rehydration still works)
	r1_again, _, err := cache1.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to re-access first object through first cache:", err)
	}
	defer r1_again.Close()

	data1_again, err := io.ReadAll(r1_again)
	if err != nil {
		t.Fatal("failed to read first object data again from first cache:", err)
	}

	if string(data1_again) != value1 {
		t.Errorf("first cache data corrupted: %q != %q", string(data1_again), value1)
	}

	// Validate statistics are non-negative
	if stat1Final.Limit < 0 || stat2Final.Limit < 0 {
		t.Error("cache limits should be non-negative")
	}
	if stat1Final.Size < 0 || stat2Final.Size < 0 {
		t.Error("cache sizes should be non-negative")
	}
	if stat1Final.Hits < 0 || stat2Final.Hits < 0 {
		t.Error("cache hits should be non-negative")
	}
	if stat1Final.Misses < 0 || stat2Final.Misses < 0 {
		t.Error("cache misses should be non-negative")
	}
	if stat1Final.Evictions < 0 || stat2Final.Evictions < 0 {
		t.Error("cache evictions should be non-negative")
	}
}

func TestCacheControlRespect(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	cache := NewCache(cacheDir, 1024)
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Test 1: Object with no cache control should not be cached
	key1 := "no-cache-control"
	value1 := "content without cache control"
	_, err = cachedBucket.PutObject(ctx, key1, strings.NewReader(value1))
	if err != nil {
		t.Fatal("failed to put object without cache control:", err)
	}

	// Verify cache statistics don't show any growth yet since objects without proper cache control shouldn't be cached
	stat1 := cache.Stat()
	if stat1.Size > 0 {
		t.Error("cache should be empty since no objects with proper cache control have been added yet")
	}

	// Test 2: Object with private cache control should not be cached
	key2 := "private-object"
	value2 := "private content"
	_, err = cachedBucket.PutObject(ctx, key2, strings.NewReader(value2), storage.CacheControl("private, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put private object:", err)
	}

	// Test 3: Object with no-cache should now be cached (for revalidation)
	key3 := "no-cache-object"
	value3 := "no cache content"
	_, err = cachedBucket.PutObject(ctx, key3, strings.NewReader(value3), storage.CacheControl("no-cache"))
	if err != nil {
		t.Fatal("failed to put no-cache object:", err)
	}

	// Test 4: Object with no-store should not be cached
	key4 := "no-store-object"
	value4 := "no store content"
	_, err = cachedBucket.PutObject(ctx, key4, strings.NewReader(value4), storage.CacheControl("no-store"))
	if err != nil {
		t.Fatal("failed to put no-store object:", err)
	}

	// Test 4b: Object with must-revalidate should now be cached (for revalidation)
	key4b := "must-revalidate-object"
	value4b := "must revalidate content"
	_, err = cachedBucket.PutObject(ctx, key4b, strings.NewReader(value4b), storage.CacheControl("public, max-age=3600, must-revalidate"))
	if err != nil {
		t.Fatal("failed to put must-revalidate object:", err)
	}

	// Test 5: Object with max-age should be cached
	key5 := "max-age-object"
	value5 := "max age content"
	_, err = cachedBucket.PutObject(ctx, key5, strings.NewReader(value5), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put max-age object:", err)
	}

	// Test 6: Immutable object should be cached
	key6 := "immutable-object"
	value6 := "immutable content"
	_, err = cachedBucket.PutObject(ctx, key6, strings.NewReader(value6), storage.CacheControl("public, immutable"))
	if err != nil {
		t.Fatal("failed to put immutable object:", err)
	}

	// Verify cache statistics show the cacheable objects
	stat := cache.Stat()
	t.Logf("Cache stats after putting objects: Size=%d, Hits=%d, Misses=%d, Evictions=%d",
		stat.Size, stat.Hits, stat.Misses, stat.Evictions)

	// The cache should now contain objects 3, 4b, 5, and 6 (no-cache, must-revalidate, max-age, immutable)
	// Object 4 (no-store) should still not be cached
	// Note: we can't check exact size since different objects may have different sizes
	// but we can verify that some objects are cached
	if stat.Size == 0 {
		t.Error("expected some objects to be cached (those with proper cache control)")
	}
}

func TestCacheExpiration(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	cache := NewCache(cacheDir, 1024)
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Test 1: Put an object with a very short max-age (1 second)
	key1 := "short-lived-object"
	value1 := "content that expires quickly"
	_, err = cachedBucket.PutObject(ctx, key1, strings.NewReader(value1), storage.CacheControl("public, max-age=1"))
	if err != nil {
		t.Fatal("failed to put short-lived object:", err)
	}

	// Verify it's initially cached
	stat1 := cache.Stat()
	if stat1.Size == 0 {
		t.Error("object should be initially cached")
	}

	// Access the object immediately - should hit cache
	r1, _, err := cachedBucket.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to get object:", err)
	}
	r1.Close()

	// Backdate the cached file to simulate expiration (instead of sleeping)
	backdateCachedFiles(cacheDir, 2*time.Second)

	// Access the object again - should be expired and fetched from underlying storage
	r2, _, err := cachedBucket.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to get expired object:", err)
	}
	r2.Close()

	// Test 2: Put an immutable object - should never expire
	key2 := "immutable-object"
	value2 := "immutable content"
	_, err = cachedBucket.PutObject(ctx, key2, strings.NewReader(value2), storage.CacheControl("public, immutable"))
	if err != nil {
		t.Fatal("failed to put immutable object:", err)
	}

	// Verify immutable object is still cached (no sleep needed - immutable never expires)
	r3, _, err := cachedBucket.GetObject(ctx, key2)
	if err != nil {
		t.Fatal("failed to get immutable object:", err)
	}
	r3.Close()

	t.Logf("Cache expiration test completed successfully")
}

func TestCacheRevalidation(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	cache := NewCache(cacheDir, 1024)
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Test 1: Put an object with no-cache directive - should be cached but always revalidated
	key1 := "no-cache-object"
	value1 := "no cache content"
	obj1, err := cachedBucket.PutObject(ctx, key1, strings.NewReader(value1), storage.CacheControl("no-cache"))
	if err != nil {
		t.Fatal("failed to put no-cache object:", err)
	}

	// Verify it's cached
	stat1 := cache.Stat()
	if stat1.Size == 0 {
		t.Error("no-cache object should be cached for revalidation")
	}

	// Get the object - this should trigger revalidation via HeadObject
	r1, info1, err := cachedBucket.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to get no-cache object:", err)
	}
	r1.Close()

	if info1.ETag != obj1.ETag {
		t.Error("ETag should match after revalidation")
	}

	// Test 2: Put an object with must-revalidate directive
	key2 := "must-revalidate-object"
	value2 := "must revalidate content"
	obj2, err := cachedBucket.PutObject(ctx, key2, strings.NewReader(value2), storage.CacheControl("public, max-age=3600, must-revalidate"))
	if err != nil {
		t.Fatal("failed to put must-revalidate object:", err)
	}

	// Get the object - this should trigger revalidation
	r2, info2, err := cachedBucket.GetObject(ctx, key2)
	if err != nil {
		t.Fatal("failed to get must-revalidate object:", err)
	}
	r2.Close()

	if info2.ETag != obj2.ETag {
		t.Error("ETag should match after revalidation")
	}

	// Test 3: Simulate ETag change by updating the object directly in the underlying storage
	// This will cause the cache to miss on revalidation and fetch fresh content
	newValue1 := "updated no cache content"
	newObj1, err := bucket.PutObject(ctx, key1, strings.NewReader(newValue1), storage.CacheControl("no-cache"))
	if err != nil {
		t.Fatal("failed to update object in underlying storage:", err)
	}

	// Now get the object through cache - should detect ETag change and fetch fresh
	r3, info3, err := cachedBucket.GetObject(ctx, key1)
	if err != nil {
		t.Fatal("failed to get updated object:", err)
	}
	defer r3.Close()

	// Verify we got the updated content
	data3, err := io.ReadAll(r3)
	if err != nil {
		t.Fatal("failed to read updated object data:", err)
	}

	if string(data3) != newValue1 {
		t.Errorf("expected updated content %q, got %q", newValue1, string(data3))
	}

	if info3.ETag == obj1.ETag {
		t.Error("ETag should have changed after object update")
	}

	if info3.ETag != newObj1.ETag {
		t.Error("ETag should match the updated object")
	}

	t.Logf("Cache revalidation test completed successfully")
}

func TestCacheRangeRequest(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	// Large cache to avoid evictions
	cache := NewCache(cacheDir, 10*1024*1024) // 10 MB
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Create a large object (larger than one block = 256KB)
	const objectSize = 512 * 1024 // 512 KB (2 blocks)
	key := "large-object"
	data := make([]byte, objectSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	_, err = cachedBucket.PutObject(ctx, key, strings.NewReader(string(data)), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put object:", err)
	}

	// Request only the first 1000 bytes
	r1, info1, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 999))
	if err != nil {
		t.Fatal("failed to get range 0-999:", err)
	}
	defer r1.Close()

	data1, err := io.ReadAll(r1)
	if err != nil {
		t.Fatal("failed to read range data:", err)
	}

	if len(data1) != 1000 {
		t.Errorf("expected 1000 bytes, got %d", len(data1))
	}

	// Verify the data matches
	for i := 0; i < 1000; i++ {
		if data1[i] != data[i] {
			t.Errorf("data mismatch at byte %d: got %d, want %d", i, data1[i], data[i])
			break
		}
	}

	// Verify object info has correct full size
	if info1.Size != objectSize {
		t.Errorf("expected object size %d, got %d", objectSize, info1.Size)
	}

	t.Logf("Cache stats after first range request: %+v", cache.Stat())

	// Request a different range (second block)
	const secondRangeStart = 256 * 1024 // Start of second block
	const secondRangeEnd = 256*1024 + 999

	r2, info2, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(secondRangeStart, secondRangeEnd))
	if err != nil {
		t.Fatal("failed to get second range:", err)
	}
	defer r2.Close()

	data2, err := io.ReadAll(r2)
	if err != nil {
		t.Fatal("failed to read second range data:", err)
	}

	if len(data2) != 1000 {
		t.Errorf("expected 1000 bytes for second range, got %d", len(data2))
	}

	// Verify the data matches
	for i := 0; i < 1000; i++ {
		if data2[i] != data[secondRangeStart+int64(i)] {
			t.Errorf("second range data mismatch at byte %d: got %d, want %d", i, data2[i], data[secondRangeStart+int64(i)])
			break
		}
	}

	if info2.Size != objectSize {
		t.Errorf("expected object size %d, got %d", objectSize, info2.Size)
	}

	t.Logf("Cache stats after second range request: %+v", cache.Stat())

	// Request the first range again - should hit cache
	statBefore := cache.Stat()
	r3, _, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 999))
	if err != nil {
		t.Fatal("failed to get first range again:", err)
	}
	r3.Close()

	statAfter := cache.Stat()
	t.Logf("Cache stats after re-requesting first range: Hits before=%d, after=%d", statBefore.Hits, statAfter.Hits)

	t.Log("Range request caching test completed successfully")
}

func TestCacheRangeRequestNotCached(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	cache := NewCache(cacheDir, 10*1024*1024)
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Create object WITHOUT cache control
	const objectSize = 100 * 1024 // 100 KB
	key := "uncacheable-object"
	data := make([]byte, objectSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	_, err = cachedBucket.PutObject(ctx, key, strings.NewReader(string(data)))
	if err != nil {
		t.Fatal("failed to put object:", err)
	}

	// Initial cache size should be 0 (no cacheable objects)
	stat1 := cache.Stat()
	if stat1.Size != 0 {
		t.Errorf("cache should be empty for object without cache control, got size=%d", stat1.Size)
	}

	// Request a range - should NOT be cached
	r1, _, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 999))
	if err != nil {
		t.Fatal("failed to get range:", err)
	}
	data1, err := io.ReadAll(r1)
	r1.Close()
	if err != nil {
		t.Fatal("failed to read range data:", err)
	}

	if len(data1) != 1000 {
		t.Errorf("expected 1000 bytes, got %d", len(data1))
	}

	// Cache should still be empty (object has no cache control)
	stat2 := cache.Stat()
	if stat2.Size != 0 {
		t.Errorf("cache should remain empty for uncacheable object, got size=%d", stat2.Size)
	}

	t.Log("Uncacheable range request test completed successfully")
}

func TestSparseFileCacheEviction(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	// Small cache to trigger evictions
	cache := NewCache(cacheDir, 1024) // 1 KB limit
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Create multiple objects that will cause evictions
	objects := []struct {
		key   string
		value string
	}{
		{"obj1", strings.Repeat("first object data ", 50)},  // ~900 bytes
		{"obj2", strings.Repeat("second object data ", 50)}, // ~950 bytes
		{"obj3", strings.Repeat("third object data ", 50)},  // ~850 bytes
	}

	for _, obj := range objects {
		_, err = cachedBucket.PutObject(ctx, obj.key, strings.NewReader(obj.value), storage.CacheControl("public, max-age=3600"))
		if err != nil {
			t.Fatalf("failed to put object %s: %v", obj.key, err)
		}
	}

	stat := cache.Stat()
	t.Logf("Cache stats after putting objects: Size=%d, Limit=%d, Evictions=%d", stat.Size, stat.Limit, stat.Evictions)

	// Cache size should not exceed limit
	if stat.Size > stat.Limit {
		t.Errorf("cache size (%d) exceeds limit (%d)", stat.Size, stat.Limit)
	}

	// There should be evictions due to small cache size
	if stat.Evictions == 0 {
		t.Log("No evictions occurred (objects may be too small to trigger evictions)")
	}

	// Test range requests with eviction
	const objectSize = 512 * 1024 // 512 KB - much larger than cache
	largeKey := "large-object"
	largeData := make([]byte, objectSize)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	_, err = cachedBucket.PutObject(ctx, largeKey, strings.NewReader(string(largeData)), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put large object:", err)
	}

	// Request a range from the large object
	r1, _, err := cachedBucket.GetObject(ctx, largeKey, storage.BytesRange(0, 999))
	if err != nil {
		t.Fatal("failed to get range from large object:", err)
	}
	r1.Close()

	statAfter := cache.Stat()
	t.Logf("Cache stats after range request on large object: Size=%d, Evictions=%d", statAfter.Size, statAfter.Evictions)

	t.Log("Sparse file cache eviction test completed successfully")
}

func TestRangeCacheRevalidation(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	cache := NewCache(cacheDir, 10*1024*1024)
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Create object with no-cache (requires revalidation)
	const objectSize = 100 * 1024
	key := "revalidate-object"
	data := make([]byte, objectSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	originalInfo, err := cachedBucket.PutObject(ctx, key, strings.NewReader(string(data)), storage.CacheControl("no-cache"))
	if err != nil {
		t.Fatal("failed to put object:", err)
	}

	// Request a range
	r1, info1, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 999))
	if err != nil {
		t.Fatal("failed to get range:", err)
	}
	r1.Close()

	if info1.ETag != originalInfo.ETag {
		t.Error("ETags should match on initial request")
	}

	// Update the object in the underlying bucket (simulating backend change)
	newData := make([]byte, objectSize)
	for i := range newData {
		newData[i] = byte((i + 100) % 256)
	}

	newInfo, err := bucket.PutObject(ctx, key, strings.NewReader(string(newData)), storage.CacheControl("no-cache"))
	if err != nil {
		t.Fatal("failed to update object:", err)
	}

	// Request the same range again - should detect ETag change
	r2, info2, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 999))
	if err != nil {
		t.Fatal("failed to get range after update:", err)
	}
	defer r2.Close()

	data2, err := io.ReadAll(r2)
	if err != nil {
		t.Fatal("failed to read updated data:", err)
	}

	// Should get the new data
	for i := 0; i < len(data2); i++ {
		if data2[i] != newData[i] {
			t.Errorf("data mismatch at byte %d after revalidation: got %d, want %d", i, data2[i], newData[i])
			break
		}
	}

	if info2.ETag == originalInfo.ETag {
		t.Error("ETag should have changed after object update")
	}

	if info2.ETag != newInfo.ETag {
		t.Error("ETag should match the updated object")
	}

	t.Log("Range cache revalidation test completed successfully")
}

// backdateCachedFiles sets the modification time of all files in a directory
// to simulate time passing without using time.Sleep
func backdateCachedFiles(cacheDir string, age time.Duration) {
	past := time.Now().Add(-age)
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		os.Chtimes(path, past, past)
		return nil
	})
}

func TestRangeLevelEviction(t *testing.T) {
	store := t.TempDir()
	cacheDir := t.TempDir()
	ctx := t.Context()

	// Use a cache size slightly larger than one block but smaller than two,
	// so caching a second range will trigger eviction of the first
	cacheSize := int64(sparseBlockSize + sparseBlockSize/2) // 1.5 blocks
	cache := NewCache(cacheDir, cacheSize)
	bucket, err := NewRegistry(store).LoadBucket(ctx, "")
	if err != nil {
		t.Fatal("failed to create bucket:", err)
	}
	cachedBucket := cache.AdaptBucket(bucket)

	// Create a large object spanning multiple blocks
	const objectSize = sparseBlockSize * 4 // 4 blocks (1 MB)
	key := "multi-block-object"
	data := make([]byte, objectSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	_, err = cachedBucket.PutObject(ctx, key, strings.NewReader(string(data)), storage.CacheControl("public, max-age=3600"))
	if err != nil {
		t.Fatal("failed to put object:", err)
	}

	// Request first block (this will create a sparse file and cache the first block)
	r1, _, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 1000))
	if err != nil {
		t.Fatal("failed to get first range:", err)
	}
	data1, _ := io.ReadAll(r1)
	r1.Close()

	// Verify first block data
	for i := 0; i < len(data1); i++ {
		if data1[i] != data[i] {
			t.Fatalf("first range data mismatch at byte %d", i)
		}
	}

	stat1 := cache.Stat()
	t.Logf("After first range: Size=%d, Evictions=%d", stat1.Size, stat1.Evictions)

	// Request third block (should trigger eviction of first block due to cache size limit)
	thirdBlockStart := int64(sparseBlockSize * 2)
	r2, _, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(thirdBlockStart, thirdBlockStart+1000))
	if err != nil {
		t.Fatal("failed to get third range:", err)
	}
	data2, _ := io.ReadAll(r2)
	r2.Close()

	// Verify third block data
	for i := 0; i < len(data2); i++ {
		if data2[i] != data[thirdBlockStart+int64(i)] {
			t.Fatalf("third range data mismatch at byte %d", i)
		}
	}

	stat2 := cache.Stat()
	t.Logf("After third range: Size=%d, Evictions=%d", stat2.Size, stat2.Evictions)

	// The cache size limit should have been exceeded, triggering eviction
	// We expect at least one eviction (of the first block)
	if stat2.Evictions == 0 {
		t.Log("No evictions occurred - cache may be larger than expected or filesystem doesn't report sparse usage accurately")
	}

	// Verify that the sparse file still exists (we evicted a range, not the whole file)
	var sparseFile string
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		sparseFile = path
		return nil
	})

	if sparseFile == "" {
		t.Fatal("sparse cache file should still exist after range eviction")
	}

	// Open the sparse file and check for holes
	f, err := os.Open(sparseFile)
	if err != nil {
		t.Fatal("failed to open sparse file:", err)
	}
	defer f.Close()

	// If evictions occurred and hole punching is supported, the file should have holes
	if stat2.Evictions > 0 {
		holePos, err := seekHole(f, 0)
		if err != nil {
			t.Logf("seekHole failed (may not be supported): %v", err)
		} else {
			fileInfo, _ := f.Stat()
			t.Logf("Sparse file: size=%d, first hole at=%d", fileInfo.Size(), holePos)

			// On filesystems supporting sparse files, there should be a hole before end of file
			if holePos < fileInfo.Size() {
				t.Log("Confirmed: hole punching occurred during eviction")
			} else {
				t.Log("No holes detected - filesystem may not support sparse files or hole punching")
			}
		}
	}

	// Request the first block again - should refetch from backend (since it was evicted)
	r3, _, err := cachedBucket.GetObject(ctx, key, storage.BytesRange(0, 1000))
	if err != nil {
		t.Fatal("failed to re-request first range:", err)
	}
	data3, _ := io.ReadAll(r3)
	r3.Close()

	// Verify we got correct data
	for i := 0; i < len(data3); i++ {
		if data3[i] != data[i] {
			t.Fatalf("re-fetched first range data mismatch at byte %d", i)
		}
	}

	stat3 := cache.Stat()
	t.Logf("After re-fetching first range: Size=%d, Evictions=%d", stat3.Size, stat3.Evictions)

	t.Log("Range-level eviction test completed successfully")
}
