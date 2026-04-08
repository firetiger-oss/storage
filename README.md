# storage [![CI](https://github.com/firetiger-oss/storage/actions/workflows/ci.yml/badge.svg)](https://github.com/firetiger-oss/storage/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/firetiger-oss/storage.svg)](https://pkg.go.dev/github.com/firetiger-oss/storage)

Batteries-included toolkit for building applications on top of object storage in Go.

## Motivation

Object storage is one of the most powerful building blocks available to
application developers — infinitely scalable, durable, and cheap. Yet building
on it in Go means picking a provider SDK, wiring up retries, caching, and
observability, and hoping you never need to swap backends or test offline.

The `storage` package removes that friction. A single
[`Bucket`](https://pkg.go.dev/github.com/firetiger-oss/storage#Bucket) interface
gives you S3, Google Cloud Storage, the local file system, HTTP, and in-memory
storage through one API — pick a URI, import a driver, and go. On top of that
foundation the package ships composable adapters for caching, prefixing,
instrumentation, read-only access, and more, so the pieces you usually have to
build yourself are already there. Streaming operations return `iter.Seq2`
iterators that plug straight into range loops and the standard library, keeping
everything idiomatic and zero-allocation where it counts.

Whether you are building a data pipeline, a media service, or a CLI tool that
needs to talk to the cloud, `storage` is designed to let you focus on your
application instead of the plumbing underneath it.

## Usage

### [storage.LoadBucket](https://pkg.go.dev/github.com/firetiger-oss/storage#LoadBucket)

Load a bucket by URI. The scheme selects the backend — import the backend
package for side effects to register it.

```go
import (
    "github.com/firetiger-oss/storage"
    _ "github.com/firetiger-oss/storage/s3"  // register s3:// scheme
    _ "github.com/firetiger-oss/storage/gs"  // register gs:// scheme
    _ "github.com/firetiger-oss/storage/file" // register file:// scheme
)

bucket, err := storage.LoadBucket(ctx, "s3://my-bucket")
```

### [storage.GetObject](https://pkg.go.dev/github.com/firetiger-oss/storage#GetObject) / [storage.PutObject](https://pkg.go.dev/github.com/firetiger-oss/storage#PutObject)

Top-level convenience functions operate directly on object URIs without
loading a bucket first.

```go
// Write an object
_, err := storage.PutObject(ctx, "s3://my-bucket/path/to/file.txt",
    strings.NewReader("Hello, World!"),
    storage.ContentType("text/plain"),
)

// Read it back
reader, info, err := storage.GetObject(ctx, "s3://my-bucket/path/to/file.txt")
defer reader.Close()
```

### [storage.ListObjects](https://pkg.go.dev/github.com/firetiger-oss/storage#ListObjects)

List objects under a prefix. Results stream as an iterator.

```go
for object, err := range storage.ListObjects(ctx, "s3://my-bucket/logs/") {
    if err != nil {
        return err
    }
    fmt.Printf("%s (%d bytes)\n", object.Key, object.Size)
}
```

### Backends

| Backend | URI | Import |
|---------|-----|--------|
| Amazon S3 | `s3://bucket/prefix` | `_ "github.com/firetiger-oss/storage/s3"` |
| Google Cloud Storage | `gs://bucket/prefix` | `_ "github.com/firetiger-oss/storage/gs"` |
| Local file system | `file:///path` | `_ "github.com/firetiger-oss/storage/file"` |
| In-memory | `:memory:` | `_ "github.com/firetiger-oss/storage/memory"` |
| HTTP (S3-compatible) | `http://host/path` | `_ "github.com/firetiger-oss/storage/http"` |

### [storage.AdaptBucket](https://pkg.go.dev/github.com/firetiger-oss/storage#AdaptBucket)

Wrap a bucket with adapters to add caching, prefixing, instrumentation,
or read-only protection.

```go
bucket = storage.AdaptBucket(bucket,
    storage.WithPrefix("data/"),
    storage.NewCache(),
    storage.WithInstrumentation(),
)

readOnly := storage.ReadOnlyBucket(bucket)
```

## Contributing

Contributions are welcome! To get started:

1. Ensure you have Go 1.25+ installed
2. Run `go test ./...` to verify tests pass

Please report bugs and feature requests via [GitHub Issues](https://github.com/firetiger-oss/storage/issues).

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
