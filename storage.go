package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/firetiger-oss/storage/uri"
)

var (
	ErrBucketExist         = errors.New("bucket exist")
	ErrBucketNotFound      = errors.New("bucket not found")
	ErrBucketReadOnly      = errors.New("read-only bucket")
	ErrObjectNotFound      = errors.New("object not found")
	ErrObjectNotMatch      = errors.New("object mismatch")
	ErrInvalidObjectKey    = errors.New("invalid object key")
	ErrInvalidObjectTag    = errors.New("invalid object tag")
	ErrInvalidRange        = errors.New("offset out of range")
	ErrPresignNotSupported = errors.New("presigned URLs not supported")
)

const (
	ContentTypeJSON    = "application/json"
	ContentTypeAvro    = "application/avro"
	ContentTypeParquet = "application/vnd.apache.parquet"
)

// Object is the type of values returned by the ListObjects method.
//
// This type contains the minimal set of information available about each
// object key when iterating through a prefix of the object store.
type Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last-modified,omitzero"`
}

// ObjectInfo represent detailed metadata about an object.
//
// This type differs from Object by not including the key, which is always
// known to the application when obtaining an ObjectInfo, and by including
// more metadata that are not available when iterating through a prefix of the
// object store.
type ObjectInfo struct {
	CacheControl    string            `json:"cache-control,omitempty"`
	ContentType     string            `json:"content-type,omitempty"`
	ContentEncoding string            `json:"content-encoding,omitempty"`
	ETag            string            `json:"etag,omitempty"`
	Size            int64             `json:"size"`
	LastModified    time.Time         `json:"last-modified,omitzero"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Bucket is an interface describing an object storage bucket. It is
// modeled off of the S3 object storage API. While it has many
// implementations, it uses S3's interface as the common denominator,
// because that is standard in the industry for object storage.
type Bucket interface {
	// Location returns a URI for the bucket. It always includes a
	// scheme component, and may include a path component.
	//
	// Some example location values:
	//
	//   s3://some-bucket
	//   gcs://another-one/with-prefix
	//
	// As a special exception, the "memory" bucket implementation
	// does _not_ contain a scheme prefix, and instead has the
	// special hostname of ":memory:", for historical reasons.
	Location() string

	// Access verifies that the bucket is accessible. It returns
	// nil error only if the bucket can be reached. This can be
	// used to test bucket existence and authentication.
	Access(ctx context.Context) error

	// Create instantiates a new bucket at Location().
	Create(ctx context.Context) error

	// HeadObject retrieves metadata about the object stored at
	// key.
	HeadObject(ctx context.Context, key string) (ObjectInfo, error)

	// GetObject retrieves the contents of the object stored at
	// key, as well as its metadata.
	GetObject(ctx context.Context, key string, options ...GetOption) (io.ReadCloser, ObjectInfo, error)

	// PutObject stores bytes at key.
	PutObject(ctx context.Context, key string, value io.Reader, options ...PutOption) (ObjectInfo, error)

	// DeleteObject removes whatever is found at key. It returns
	// an error if there is nothing there.
	DeleteObject(ctx context.Context, key string) error

	// DeleteObjects is like DeleteObject, but for multiple keys
	// at once.
	DeleteObjects(ctx context.Context, keys []string) error

	// ListObjects gathers a list of abbreviated metadata for all
	// objects in a bucket, or under a key prefix (set through a
	// ListOption).
	ListObjects(ctx context.Context, options ...ListOption) iter.Seq2[Object, error]

	// WatchObjects is list ListObjects but the sequence doesn't end.
	// After listing the objects, it watches for any changes to the
	// prefix and yields new objects as they are added, modified, or
	// removed. The removal of objects is indicated by yielding an
	// Object with a negative Size.
	WatchObjects(ctx context.Context, options ...ListOption) iter.Seq2[Object, error]

	// PresignGetObject generates a presigned URL for getting an object.
	PresignGetObject(ctx context.Context, key string, options ...GetOption) (string, error)

	// PresignPutObject generates a presigned URL for putting an object.
	PresignPutObject(ctx context.Context, key string, options ...PutOption) (string, error)

	// PresignHeadObject generates a presigned URL for getting object metadata.
	PresignHeadObject(ctx context.Context, key string) (string, error)

	// PresignDeleteObject generates a presigned URL for deleting an object.
	PresignDeleteObject(ctx context.Context, key string) (string, error)
}

type Adapter interface {
	AdaptBucket(Bucket) Bucket
}

type AdapterFunc func(Bucket) Bucket

func (a AdapterFunc) AdaptBucket(b Bucket) Bucket { return a(b) }

func AdaptBucket(bucket Bucket, adapters ...Adapter) Bucket {
	for _, adapter := range adapters {
		bucket = adapter.AdaptBucket(bucket)
	}
	return bucket
}

type Registry interface {
	LoadBucket(ctx context.Context, bucketURI string) (Bucket, error)
}

type RegistryFunc func(context.Context, string) (Bucket, error)

func (reg RegistryFunc) LoadBucket(ctx context.Context, bucketURI string) (Bucket, error) {
	return reg(ctx, bucketURI)
}

func SingleBucketRegistry(bucket Bucket) Registry {
	return RegistryFunc(func(ctx context.Context, bucketURI string) (Bucket, error) {
		bucketType, bucketName, objectURI := uri.Split(bucketURI)
		bucketLocation := uri.Join(bucketType, bucketName)
		if bucketLocation != bucket.Location() {
			return nil, fmt.Errorf("%s: %w (only has %s)", bucketURI, ErrBucketNotFound, bucket.Location())
		}
		return normalizeBucket(bucket, bucketType, objectURI), nil
	})
}

var (
	globalMutex    sync.RWMutex
	globalAdapters []Adapter
	globalRegistry = map[string]Registry{}
)

func WithScheme(scheme string) Adapter {
	return AdapterFunc(func(b Bucket) Bucket {
		return &typedBucket{
			Bucket:     b,
			bucketType: scheme,
		}
	})
}

type typedBucket struct {
	Bucket
	bucketType string
}

func (b *typedBucket) Location() string {
	_, location, prefix := uri.Split(b.Bucket.Location())
	return uri.Join(b.bucketType, location, prefix)
}

func Register(typ string, reg Registry) {
	globalMutex.Lock()
	globalRegistry[typ] = reg
	globalMutex.Unlock()
}

func Install(adapters ...Adapter) {
	globalMutex.Lock()
	globalAdapters = append(globalAdapters, adapters...)
	globalMutex.Unlock()
}

func DefaultRegistry() Registry {
	return RegistryFunc(LoadBucket)
}

func LoadBucket(ctx context.Context, bucketURI string) (Bucket, error) {
	bucketType, bucketName, objectKey := uri.Split(bucketURI)
	globalMutex.RLock()
	bucketAdapters := globalAdapters
	bucketRegistry, ok := globalRegistry[bucketType]
	globalMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%s: %w (did you forget the import?)", bucketURI, ErrBucketNotFound)
	}
	bucket, err := bucketRegistry.LoadBucket(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	bucket = normalizeBucket(bucket, bucketType, objectKey)
	bucket = AdaptBucket(bucket, bucketAdapters...)
	return bucket, nil
}

func normalizeBucket(bucket Bucket, bucketType, objectKey string) Bucket {
	if objectKey != "" {
		if !strings.HasSuffix(objectKey, "/") {
			objectKey += "/"
		}
		bucket = WithPrefix(objectKey).AdaptBucket(bucket)
	}
	if bucketType != "" {
		bucket = WithScheme(bucketType).AdaptBucket(bucket)
	}
	return bucket
}

func HeadObject(ctx context.Context, objectURI string) (ObjectInfo, error) {
	return HeadObjectAt(ctx, DefaultRegistry(), objectURI)
}

func HeadObjectAt(ctx context.Context, registry Registry, objectURI string) (ObjectInfo, error) {
	bucketType, bucketName, objectKey := uri.Split(objectURI)
	bucket, err := registry.LoadBucket(ctx, uri.Join(bucketType, bucketName))
	if err != nil {
		return ObjectInfo{}, err
	}
	return bucket.HeadObject(ctx, objectKey)
}

func GetObject(ctx context.Context, objectURI string, options ...GetOption) (io.ReadCloser, ObjectInfo, error) {
	return GetObjectAt(ctx, DefaultRegistry(), objectURI, options...)
}

func GetObjectAt(ctx context.Context, registry Registry, objectURI string, options ...GetOption) (io.ReadCloser, ObjectInfo, error) {
	bucketType, bucketName, objectKey := uri.Split(objectURI)
	bucket, err := registry.LoadBucket(ctx, uri.Join(bucketType, bucketName))
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return bucket.GetObject(ctx, objectKey, options...)
}

func PutObject(ctx context.Context, objectURI string, object io.Reader, options ...PutOption) (ObjectInfo, error) {
	return PutObjectAt(ctx, DefaultRegistry(), objectURI, object, options...)
}

func PutObjectAt(ctx context.Context, registry Registry, objectURI string, object io.Reader, options ...PutOption) (ObjectInfo, error) {
	bucketType, bucketName, objectKey := uri.Split(objectURI)
	bucket, err := registry.LoadBucket(ctx, uri.Join(bucketType, bucketName))
	if err != nil {
		return ObjectInfo{}, err
	}
	return bucket.PutObject(ctx, objectKey, object, options...)
}

func PutObjectWriter(ctx context.Context, objectURI string, options ...PutOption) io.WriteCloser {
	return PutObjectAtWriter(ctx, DefaultRegistry(), objectURI, options...)
}

// PutObjectAtWriter wraps a storage.PutObjectAt call in a WriteCloser.
// Errors returned by PutObjectAt are passed through the WriteCloser methods.
func PutObjectAtWriter(ctx context.Context, registry Registry, objectURI string, options ...PutOption) io.WriteCloser {
	pr, pw := io.Pipe()
	ow := &objectWriter{
		pw:    pw,
		errCh: make(chan error, 1),
	}

	go func() {
		defer close(ow.errCh)

		_, err := PutObjectAt(ctx, registry, objectURI, pr, options...)
		_ = pr.CloseWithError(err)
		ow.errCh <- err
	}()

	return ow
}

type objectWriter struct {
	pw    *io.PipeWriter
	errCh chan error
	err   error
	once  sync.Once
}

func (ow *objectWriter) Close() error {
	_ = ow.pw.Close()
	ow.once.Do(func() {
		ow.err = <-ow.errCh
	})
	return ow.err
}

func (ow *objectWriter) Write(p []byte) (int, error) {
	return ow.pw.Write(p)
}

func DeleteObject(ctx context.Context, objectURI string) error {
	return DeleteObjectAt(ctx, DefaultRegistry(), objectURI)
}

func DeleteObjectAt(ctx context.Context, registry Registry, objectURI string) error {
	bucketType, bucketName, objectKey := uri.Split(objectURI)
	bucket, err := registry.LoadBucket(ctx, uri.Join(bucketType, bucketName))
	if err != nil {
		return err
	}
	return bucket.DeleteObject(ctx, objectKey)
}

func DeleteObjects(ctx context.Context, objectURIs []string) error {
	return DeleteObjectsAt(ctx, DefaultRegistry(), objectURIs)
}

func DeleteObjectsAt(ctx context.Context, registry Registry, objectURIs []string) error {
	type bucketURI struct {
		bucketType, bucketName string
	}

	type bucketDeletes struct {
		bucket Bucket
		keys   []string
	}

	if len(objectURIs) == 0 {
		return nil
	}

	deletes := map[bucketURI]*bucketDeletes{}

	for _, objectURI := range objectURIs {
		bucketType, bucketName, objectKey := uri.Split(objectURI)
		bucket, err := registry.LoadBucket(ctx, uri.Join(bucketType, bucketName))
		if err != nil {
			return err
		}
		bucketKey := bucketURI{bucketType, bucketName}
		if del, ok := deletes[bucketKey]; ok {
			del.keys = append(del.keys, objectKey)
		} else {
			deletes[bucketKey] = &bucketDeletes{
				bucket: bucket,
				keys:   []string{objectKey},
			}
		}
	}

	waitGroup := sync.WaitGroup{}
	errChan := make(chan error)

	for _, del := range deletes {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errChan <- del.bucket.DeleteObjects(ctx, del.keys)
		}()
	}

	go func() {
		waitGroup.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func Location(location, path string) string {
	scheme, location, base := uri.Split(location)
	return uri.Join(scheme, location, base, path)
}

func ValidObjectKey(key string) error {
	if !fs.ValidPath(key) {
		return fmt.Errorf("%w (%s)", ErrInvalidObjectKey, key)
	}
	return nil
}

func ValidObjectRange(key string, start, end int64) error {
	if start < 0 || end < 0 || end < start {
		return fmt.Errorf("%s: %w (start=%d, end=%d)", key, ErrInvalidRange, start, end)
	}
	return nil
}

func ListObjects(ctx context.Context, prefixURI string, options ...ListOption) iter.Seq2[Object, error] {
	return ListObjectsAt(ctx, DefaultRegistry(), prefixURI, options...)
}

func ListObjectsAt(ctx context.Context, registry Registry, prefixURI string, options ...ListOption) iter.Seq2[Object, error] {
	return func(yield func(Object, error) bool) {
		bucketType, bucketName, objectPrefix := uri.Split(prefixURI)
		bucket, err := registry.LoadBucket(ctx, uri.Join(bucketType, bucketName))
		if err != nil {
			yield(Object{}, err)
			return
		}

		listOptions := slices.Clip(options)
		if objectPrefix != "" {
			listOptions = append(listOptions, KeyPrefix(objectPrefix))
		}

		for object, err := range bucket.ListObjects(ctx, listOptions...) {
			if err != nil {
				yield(Object{}, err)
				return
			}
			object.Key = uri.Join(bucketType, bucketName, object.Key)
			if !yield(object, nil) {
				return
			}
		}
	}
}
