package gs

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/firetiger-oss/storage"
)

// TestSignedGetOptionsIncludesAcceptEncodingGzip ensures the signed GET
// options commit the client to sending Accept-Encoding: gzip so GCS
// returns gzip-encoded objects as-stored rather than decompressing
// them on the fly. Without this header in the signature, a presigned
// URL hit directly or through http.BucketHandler's presign-redirect
// path would trigger transcoding again, contradicting the
// ReadCompressed(true) behaviour of the direct GetObject path.
func TestSignedGetOptionsIncludesAcceptEncodingGzip(t *testing.T) {
	opts, err := signedGetOptions("key", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !hasHeader(opts.Headers, "Accept-Encoding:gzip") {
		t.Errorf("opts.Headers = %v; want to include Accept-Encoding:gzip", opts.Headers)
	}
}

// TestSignedGetOptionsIncludesRangeAndAcceptEncoding makes sure the
// Range and Accept-Encoding headers are both signed when a BytesRange
// option is supplied, so presigned tail reads work without triggering
// transcoding.
func TestSignedGetOptionsIncludesRangeAndAcceptEncoding(t *testing.T) {
	opts, err := signedGetOptions("key", time.Hour, storage.BytesRange(100, -1))
	if err != nil {
		t.Fatal(err)
	}
	if !hasHeader(opts.Headers, "Accept-Encoding:gzip") {
		t.Errorf("opts.Headers = %v; want to include Accept-Encoding:gzip", opts.Headers)
	}
	if !hasHeader(opts.Headers, "Range:bytes=100-") {
		t.Errorf("opts.Headers = %v; want to include Range:bytes=100-", opts.Headers)
	}
}

func hasHeader(headers []string, want string) bool {
	return slices.ContainsFunc(headers, func(h string) bool {
		return strings.EqualFold(h, want)
	})
}
