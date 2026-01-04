package s3_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/firetiger-oss/storage"
	storages3 "github.com/firetiger-oss/storage/s3"
	"github.com/firetiger-oss/storage/s3/fakes3"
	storagetest "github.com/firetiger-oss/storage/test"
)

func TestS3(t *testing.T) {
	storagetest.TestStorage(t, func(*testing.T) (storage.Bucket, error) {
		bucket := "test"
		client := fakes3.NewClient(bucket)
		return storages3.NewBucket(client, bucket), nil
	})
}

// mockRangeClient captures GetObject inputs for testing Range header formatting.
type mockRangeClient struct {
	storages3.Client
	capturedRange string
}

func (m *mockRangeClient) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if params.Range != nil {
		m.capturedRange = aws.ToString(params.Range)
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("test")),
	}, nil
}

// TestS3RangeHeaderFormat verifies that the S3 client correctly formats
// Range headers for closed ranges (bytes=0-100).
//
// Note: Open-ended ranges (end=-1) are validated and rejected by GetObject before
// reaching newGetObjectInput. The open-ended range fix (producing "bytes=N-" instead
// of "bytes=N--1") is tested via the HTTP storage tests which cover the same pattern.
// The HTTP server's presign path passes end=-1 through to the bucket without validation,
// and both HTTP and S3 bucket implementations use the same fix pattern.
func TestS3RangeHeaderFormat(t *testing.T) {
	tests := []struct {
		name          string
		start         int64
		end           int64
		expectedRange string
	}{
		{
			name:          "closed range from start",
			start:         0,
			end:           100,
			expectedRange: "bytes=0-100",
		},
		{
			name:          "closed range with offset",
			start:         512,
			end:           1023,
			expectedRange: "bytes=512-1023",
		},
		{
			name:          "single byte range",
			start:         42,
			end:           42,
			expectedRange: "bytes=42-42",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockRangeClient{}
			bucket := storages3.NewBucket(mock, "test-bucket")

			_, _, err := bucket.GetObject(t.Context(), "test-key", storage.BytesRange(tc.start, tc.end))
			if err != nil {
				t.Fatalf("GetObject failed: %v", err)
			}

			if mock.capturedRange != tc.expectedRange {
				t.Errorf("expected Range %q, got %q", tc.expectedRange, mock.capturedRange)
			}
		})
	}
}
