package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/firetiger-oss/storage"
)

func TestParseBytesRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *bytesRange
		wantErr bool
	}{
		{
			name:    "valid range",
			input:   "bytes=0-10",
			want:    &bytesRange{start: 0, end: 10},
			wantErr: false,
		},
		{
			name:    "open ended range",
			input:   "bytes=5-",
			want:    &bytesRange{start: 5, end: -1},
			wantErr: false,
		},
		{
			name:    "invalid prefix",
			input:   "invalid=0-10",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid range format",
			input:   "bytes=abc-def",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "start greater than end",
			input:   "bytes=10-5",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "negative start open ended",
			input:   "bytes=-1",
			want:    &bytesRange{start: -1, end: -1},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBytesRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBytesRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && tt.want != nil {
				if got.start != tt.want.start || got.end != tt.want.end {
					t.Errorf("parseBytesRange() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestBytesRangeContentRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		size    int64
		want    string
		wantErr bool
	}{
		{
			name:    "valid range",
			input:   "bytes=0-10",
			size:    10,
			want:    "bytes 0-10/10",
			wantErr: false,
		},
		{
			name:    "open ended range",
			input:   "bytes=5-",
			size:    10,
			want:    "bytes 5-9/10",
			wantErr: false,
		},
		{
			name:    "negative start",
			input:   "bytes=-10",
			size:    10,
			want:    "bytes 0-9/10",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpRange, err := parseBytesRange(tt.input)
			if err != nil {
				t.Errorf("parseBytesRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got := httpRange.ContentRange(tt.size)
			if got != tt.want {
				t.Errorf("ContentRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

// presignRedirectBucket is a mock bucket that returns ErrPresignRedirect for all operations
// and provides presigned URLs.
type presignRedirectBucket struct {
	storage.Bucket
	presignedURL string
}

func (b *presignRedirectBucket) Location() string {
	return "test://bucket"
}

func (b *presignRedirectBucket) HeadObject(ctx context.Context, key string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, storage.ErrPresignRedirect
}

func (b *presignRedirectBucket) GetObject(ctx context.Context, key string, options ...storage.GetOption) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, storage.ErrPresignRedirect
}

func (b *presignRedirectBucket) PutObject(ctx context.Context, key string, value io.Reader, options ...storage.PutOption) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, storage.ErrPresignRedirect
}

func (b *presignRedirectBucket) DeleteObject(ctx context.Context, key string) error {
	return storage.ErrPresignRedirect
}

func (b *presignRedirectBucket) PresignGetObject(ctx context.Context, key string, expiration time.Duration, options ...storage.GetOption) (string, error) {
	return b.presignedURL, nil
}

func (b *presignRedirectBucket) PresignPutObject(ctx context.Context, key string, expiration time.Duration, options ...storage.PutOption) (string, error) {
	return b.presignedURL, nil
}

func (b *presignRedirectBucket) PresignHeadObject(ctx context.Context, key string) (string, error) {
	return b.presignedURL, nil
}

func (b *presignRedirectBucket) PresignDeleteObject(ctx context.Context, key string) (string, error) {
	return b.presignedURL, nil
}

func TestBucketHandlerPresignRedirectGET(t *testing.T) {
	bucket := &presignRedirectBucket{presignedURL: "https://example.com/presigned-get"}
	handler := BucketHandler(bucket)

	req := httptest.NewRequest(http.MethodGet, "/test-key", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != bucket.presignedURL {
		t.Errorf("expected Location %q, got %q", bucket.presignedURL, loc)
	}
}

func TestBucketHandlerPresignRedirectHEAD(t *testing.T) {
	bucket := &presignRedirectBucket{presignedURL: "https://example.com/presigned-head"}
	handler := BucketHandler(bucket)

	req := httptest.NewRequest(http.MethodHead, "/test-key", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != bucket.presignedURL {
		t.Errorf("expected Location %q, got %q", bucket.presignedURL, loc)
	}
}

func TestBucketHandlerPresignRedirectPUT(t *testing.T) {
	bucket := &presignRedirectBucket{presignedURL: "https://example.com/presigned-put"}
	handler := BucketHandler(bucket)

	req := httptest.NewRequest(http.MethodPut, "/test-key", strings.NewReader("test content"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != bucket.presignedURL {
		t.Errorf("expected Location %q, got %q", bucket.presignedURL, loc)
	}
}

func TestBucketHandlerPresignRedirectDELETE(t *testing.T) {
	bucket := &presignRedirectBucket{presignedURL: "https://example.com/presigned-delete"}
	handler := BucketHandler(bucket)

	req := httptest.NewRequest(http.MethodDelete, "/test-key", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != bucket.presignedURL {
		t.Errorf("expected Location %q, got %q", bucket.presignedURL, loc)
	}
}

// presignFailBucket is a mock bucket that returns ErrPresignRedirect but then fails to presign.
type presignFailBucket struct {
	storage.Bucket
}

func (b *presignFailBucket) Location() string {
	return "test://bucket"
}

func (b *presignFailBucket) GetObject(ctx context.Context, key string, options ...storage.GetOption) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, storage.ErrPresignRedirect
}

func (b *presignFailBucket) PresignGetObject(ctx context.Context, key string, expiration time.Duration, options ...storage.GetOption) (string, error) {
	return "", storage.ErrPresignNotSupported
}

func TestBucketHandlerPresignRedirectPresignFails(t *testing.T) {
	bucket := &presignFailBucket{}
	handler := BucketHandler(bucket)

	req := httptest.NewRequest(http.MethodGet, "/test-key", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// When presign fails, should return the presign error (mapped to appropriate HTTP status)
	if rec.Code == http.StatusTemporaryRedirect {
		t.Error("expected non-redirect status when presign fails")
	}
}
