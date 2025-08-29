# CLAUDE.md - Repository Management Guide

This file contains information to help Claude (and other AI assistants) understand and work effectively with this repository.

## Project Overview

**Name**: storage  
**Type**: Go library/package  
**Purpose**: Unified interface for cloud object storage providers (S3, GCS, file system, memory, HTTP)  
**License**: Apache 2.0  

## Architecture

### Core Components

1. **Main Interface** (`storage.go`): Core `Bucket` interface and global functions
2. **Storage Backends**:
   - `s3/` - Amazon S3 implementation
   - `gs/` - Google Cloud Storage implementation  
   - `file/` - Local file system storage
   - `memory/` - In-memory storage for testing
   - `http/` - HTTP-based storage
3. **Supporting Systems**:
   - `cache/` - Caching layer with LRU and TTL support
   - `concurrent/` - Concurrency utilities
   - `backoff/` - Retry logic with exponential backoff
   - `uri/` - URI parsing and manipulation

### Key Patterns

- **Registry Pattern**: Each backend registers itself with `storage.Register()`
- **Adapter Pattern**: Buckets can be wrapped with adapters for additional functionality
- **Options Pattern**: Functions accept option structs for configuration
- **Iterator Pattern**: Uses Go 1.23+ `iter.Seq2` for listing operations

## Development Guidelines

### Code Style
- Follow standard Go conventions
- Use structured logging where appropriate
- Handle errors properly with context
- Write comprehensive tests for all functionality
- Use interfaces for testability

### Testing
```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run specific package tests
go test ./s3
```

### Dependencies
- **AWS SDK v2**: For S3 integration
- **Google Cloud Storage**: For GCS integration  
- **fsnotify**: For file system watching
- **OpenTelemetry**: For observability
- Standard library packages

## File Structure

```
/
├── storage.go              # Main interface and global functions
├── options.go              # Option types and constructors
├── cache.go               # Cache adapter
├── prefix.go              # Prefix adapter
├── readonly.go            # Read-only adapter  
├── mount.go               # Mount utilities
├── watch.go               # File system watching
├── instrument.go          # Instrumentation/telemetry
├── backoff/               # Retry logic
├── cache/                 # Caching implementations
├── concurrent/            # Concurrency utilities
├── file/                  # File system backend
├── gs/                    # Google Cloud Storage backend
├── http/                  # HTTP backend
├── memory/                # In-memory backend
├── s3/                    # Amazon S3 backend
├── internal/              # Internal utilities
├── test/                  # Test utilities
└── uri/                   # URI handling
```

## Common Tasks

### Adding a New Storage Backend

1. Create new package directory (e.g., `azure/`)
2. Implement `storage.Bucket` interface
3. Create registry function
4. Register in `init()` function:
   ```go
   func init() {
       storage.Register("azure", NewRegistry())
   }
   ```
5. Add comprehensive tests
6. Update README.md with new backend

### Adding New Functionality

1. Consider if it should be:
   - Core interface method (requires all backends to implement)
   - Adapter (wraps existing buckets with new behavior)
   - Utility function
2. Update relevant interfaces
3. Implement across all backends if core feature
4. Add tests and documentation

### Backend-Specific Notes

#### S3 Backend (`s3/`)
- Uses AWS SDK v2
- Supports multipart uploads
- Handles presigned URLs
- Has fake S3 server for testing (`s3/fakes3/`)

#### Google Cloud Storage (`gs/`)
- Uses Google Cloud Go SDK
- Has custom client wrapper (`gs/gsclient/`)
- Supports Google Cloud authentication

#### File System (`file/`)
- Uses extended attributes for metadata
- Supports file system watching via fsnotify
- Platform-specific implementations for Darwin/Linux

#### Memory Backend (`memory/`)
- Thread-safe in-memory implementation
- Used primarily for testing
- No persistent storage

#### HTTP Backend (`http/`)
- Read-only HTTP/HTTPS support
- Basic bucket server implementation

### Error Handling

Standard errors are defined in `storage.go`:
- `ErrBucketNotFound`: Bucket doesn't exist
- `ErrObjectNotFound`: Object doesn't exist  
- `ErrBucketReadOnly`: Write operation on read-only bucket
- etc.

Always check for these standard errors when implementing backends.

### Testing Strategy

1. **Unit Tests**: Each package has `*_test.go` files
2. **Integration Tests**: Test real backends when possible
3. **Test Utilities**: `test/` package provides mock implementations
4. **Benchmarks**: Performance testing for critical paths

## Building and Deployment

### Build
```bash
go build ./...
```

### Linting
```bash
# If available
golangci-lint run
```

### Dependencies
```bash
# Update dependencies
go mod tidy
go mod verify
```

## Troubleshooting

### Common Issues

1. **Import Errors**: Make sure to import backend packages with `_` for side effects
2. **Authentication**: Check cloud provider authentication setup
3. **URI Format**: Each backend has specific URI format requirements
4. **Context Cancellation**: Always pass and respect context for cancellation

### Debugging

1. Enable structured logging if available
2. Use context tracing for request flows
3. Check backend-specific configuration
4. Verify network connectivity for cloud backends

## Performance Considerations

- Use connection pooling for cloud backends
- Enable caching adapters where appropriate
- Consider concurrent operations for batch work
- Monitor memory usage with large objects
- Use range reads for partial object access

## Security

- Never commit credentials to repository
- Use IAM roles when possible for cloud access
- Validate object keys to prevent path traversal
- Consider encryption at rest and in transit
- Audit access patterns and permissions

## Compatibility

- Go 1.24.4+ (uses newer iterator patterns)
- Backward compatible API design
- Semantic versioning for releases
- Comprehensive test coverage for compatibility

## Maintenance

- Regular dependency updates
- Security vulnerability scanning
- Performance monitoring
- Documentation updates
- Community issue triage