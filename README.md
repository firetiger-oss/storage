# storage

[![Go Reference](https://pkg.go.dev/badge/github.com/firetiger-oss/storage.svg)](https://pkg.go.dev/github.com/firetiger-oss/storage)
[![CI](https://github.com/firetiger-oss/storage/actions/workflows/ci.yml/badge.svg)](https://github.com/firetiger-oss/storage/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Go package to interact with cloud object storage providers using a unified interface.

For full API documentation, see [pkg.go.dev](https://pkg.go.dev/github.com/firetiger-oss/storage).

## Overview

This package provides a unified interface for working with different object
storage systems, including Amazon S3, Google Cloud Storage, local file system,
and in-memory storage. The API is modeled after the S3 object storage interface
as the common denominator.

## Features

- **Unified Interface**: Single API for multiple storage backends
- **Multiple Backends**: S3, Google Cloud Storage, local file system, and in-memory storage
- **Caching**: Built-in caching layer with LRU and TTL support
- **Concurrency**: Thread-safe operations with concurrent utilities
- **Advanced Operations**: 
  - Presigned URLs
  - Range reads
  - Metadata support
  - Multipart uploads
  - Object watching (file system)
- **Adapters**: Pluggable adapter system for extending functionality
- **URI Support**: Consistent URI-based addressing across all backends

## Installation

```bash
go get github.com/firetiger-oss/storage
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/firetiger-oss/storage"
    // Import the backend you need
    _ "github.com/firetiger-oss/storage/s3"
)

func main() {
    ctx := context.Background()
    
    // Get an object
    reader, info, err := storage.GetObject(ctx, "s3://my-bucket/path/to/file.txt")
    if err != nil {
        panic(err)
    }
    defer reader.Close()
    
    // Put an object
    content := strings.NewReader("Hello, World!")
    _, err = storage.PutObject(ctx, "s3://my-bucket/path/to/new-file.txt", content)
    if err != nil {
        panic(err)
    }
    
    // List objects
    for object, err := range storage.ListObjects(ctx, "s3://my-bucket/path/") {
        if err != nil {
            panic(err)
        }
        fmt.Printf("Key: %s, Size: %d\n", object.Key, object.Size)
    }
}
```

### Working with Buckets

```go
// Load a specific bucket
bucket, err := storage.LoadBucket(ctx, "s3://my-bucket")
if err != nil {
    panic(err)
}

// Use bucket methods directly
info, err := bucket.HeadObject(ctx, "file.txt")
if err != nil {
    panic(err)
}
```

## Configuration

### AWS S3 Configuration
Follows standard AWS SDK configuration:
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, etc.)
- AWS credentials file
- IAM roles

### Google Cloud Storage Configuration
- Service account key file
- Application Default Credentials
- Environment variable `GOOGLE_APPLICATION_CREDENTIALS`

## Advanced Usage

### Options

#### Get Options
```go
// Range read
reader, info, err := storage.GetObject(ctx, "s3://bucket/file.txt", 
    storage.BytesRange(0, 1024))
```

#### Put Options
```go
content := strings.NewReader("data")
info, err := storage.PutObject(ctx, "s3://bucket/file.txt", content,
    storage.ContentType("text/plain"),
    storage.CacheControl("max-age=3600"),
    storage.Metadata("author", "example"))
```

#### List Options
```go
for object, err := range storage.ListObjects(ctx, "s3://bucket/",
    storage.KeyPrefix("logs/"),
    storage.MaxKeys(100)) {
    // Process objects
}
```

### Adapters

Create custom adapters to extend functionality:

```go
// Add caching to any bucket
cache := storage.NewCache()
cached := storage.AdaptBucket(bucket, cache)

// Add read-only protection
readOnly := storage.ReadOnlyBucket(bucket)

// Add prefix
prefixed := storage.AdaptBucket(bucket, storage.WithPrefix("data/"))

// Add instrumentation
instrumented := storage.AdaptBucket(bucket, storage.WithInstrumentation())
```

### Custom Registry

```go
// Create a custom registry
registry := storage.RegistryFunc(func(ctx context.Context, uri string) (storage.Bucket, error) {
    // Custom bucket loading logic
    return bucket, nil
})

// Use with specific registry
info, err := storage.HeadObjectAt(ctx, registry, "custom://bucket/file")
```

### Object Watching (File System Only)

```go
import _ "github.com/firetiger-oss/storage/file"

bucket, _ := storage.LoadBucket(ctx, "file://")
for object, err := range bucket.WatchObjects(ctx, storage.KeyPrefix("logs/")) {
    if err != nil {
        panic(err)
    }
    if object.Size < 0 {
        fmt.Printf("Object deleted: %s\n", object.Key)
    } else {
        fmt.Printf("Object changed: %s\n", object.Key)
    }
}
```

## Contributing

Contributions are welcome! Here's how to get started:

1. Fork the repository and clone it locally
2. Run tests: `go test ./...`
3. Code style is enforced by `gofmt` in CI
4. To add a new storage backend: create a new package, implement the `Bucket` interface, and register it via `storage.Register` in an `init()` function (see existing backends for examples)
5. Open a pull request against `main`

## License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.
