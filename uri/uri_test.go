package uri_test

import (
	"testing"

	"github.com/firetiger-oss/storage/uri"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		uri      string
		scheme   string
		location string
		path     string
	}{
		{
			uri:      "",
			scheme:   "",
			location: "",
			path:     "",
		},

		{
			uri:      "file:///path/to/object",
			scheme:   "file",
			location: "",
			path:     "path/to/object",
		},

		{
			uri:      "file:///file.json",
			scheme:   "file",
			location: "",
			path:     "file.json",
		},

		{
			uri:      "file:///",
			scheme:   "file",
			location: "",
			path:     "",
		},

		{
			uri:      "path/to/object",
			scheme:   "",
			location: "",
			path:     "path/to/object",
		},

		{
			uri:      "path/to/object//",
			scheme:   "",
			location: "",
			path:     "path/to/object/",
		},

		{
			uri:      "s3://bucket.com/path/to/object//",
			scheme:   "s3",
			location: "bucket.com",
			path:     "path/to/object/",
		},

		{
			uri:      "s3://bucket.com///path//to/object//",
			scheme:   "s3",
			location: "bucket.com",
			path:     "path/to/object/",
		},

		{
			uri:      ":memory:",
			scheme:   "",
			location: ":memory:",
			path:     "",
		},

		{
			uri:      ":memory:path/to/object",
			scheme:   "",
			location: ":memory:",
			path:     "path/to/object",
		},

		{
			uri:      ":memory:/path/to/object",
			scheme:   "",
			location: ":memory:",
			path:     "path/to/object",
		},

		{
			uri:      ":memory:///path/to/object",
			scheme:   "",
			location: ":memory:",
			path:     "path/to/object",
		},

		// Local file paths with . and .. directory indicators
		{
			uri:      "/tmp/.",
			scheme:   "file",
			location: "",
			path:     "tmp/",
		},

		{
			uri:      "/tmp/..",
			scheme:   "file",
			location: "",
			path:     "",
		},

		{
			uri:      "/tmp/foo/..",
			scheme:   "file",
			location: "",
			path:     "tmp/",
		},

		// Additional file:// test cases to verify the special behavior
		{
			uri:      "file:///",
			scheme:   "file",
			location: "",
			path:     "",
		},

		{
			uri:      "file:///usr/local/bin",
			scheme:   "file",
			location: "",
			path:     "usr/local/bin",
		},

		{
			uri:      "file:////some/path/with/leading/slash",
			scheme:   "file",
			location: "",
			path:     "some/path/with/leading/slash",
		},
	}

	for _, test := range tests {
		t.Run(test.uri, func(t *testing.T) {
			scheme, location, path := uri.Split(test.uri)
			if scheme != test.scheme {
				t.Fatalf("unexpected bucket type: %q != %q", scheme, test.scheme)
			}
			if location != test.location {
				t.Fatalf("unexpected bucket name: %q != %q", location, test.location)
			}
			if path != test.path {
				t.Fatalf("unexpected object key: %q != %q", path, test.path)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		scheme   string
		location string
		path     string
		uri      string
	}{
		{
			scheme:   "file",
			location: "",
			path:     "path/to/object",
			uri:      "file:///path/to/object",
		},

		{
			scheme:   "file",
			location: "",
			path:     "",
			uri:      "file:///",
		},

		{
			scheme:   "",
			location: "",
			path:     "path/to/object",
			uri:      "path/to/object",
		},

		{
			scheme:   "",
			location: "",
			path:     "path/to/object",
			uri:      "path/to/object",
		},

		{
			scheme:   "s3",
			location: "bucket.com",
			path:     "path/to/object",
			uri:      "s3://bucket.com/path/to/object",
		},

		{
			scheme:   "s3",
			location: "bucket.com",
			path:     "",
			uri:      "s3://bucket.com",
		},

		{
			scheme:   "s3",
			location: "bucket.com",
			path:     "///",
			uri:      "s3://bucket.com",
		},

		{
			scheme:   "",
			location: ":memory:",
			path:     "",
			uri:      ":memory:",
		},

		{
			scheme:   "",
			location: ":memory:",
			path:     "path/to/object",
			uri:      ":memory:path/to/object",
		},
	}

	for _, test := range tests {
		t.Run(test.uri, func(t *testing.T) {
			uri := uri.Join(test.scheme, test.location, test.path)
			if uri != test.uri {
				t.Fatalf("unexpected object URI: %q != %q", uri, test.uri)
			}
		})
	}
}

func TestJoinPreservesTrailingSlash(t *testing.T) {
	tests := []struct {
		name     string
		scheme   string
		location string
		path     string
		expected string
	}{
		{
			name:     "file scheme with trailing slash",
			scheme:   "file",
			location: "",
			path:     "path/to/dir/",
			expected: "file:///path/to/dir/",
		},
		{
			name:     "s3 scheme with trailing slash",
			scheme:   "s3",
			location: "bucket",
			path:     "path/to/dir/",
			expected: "s3://bucket/path/to/dir/",
		},
		{
			name:     "no scheme with trailing slash",
			scheme:   "",
			location: "",
			path:     "path/to/dir/",
			expected: "path/to/dir/",
		},
		{
			name:     "memory scheme with trailing slash",
			scheme:   "",
			location: ":memory:",
			path:     "path/to/dir/",
			expected: ":memory:path/to/dir/",
		},
		{
			name:     "multiple path segments with trailing slash on last",
			scheme:   "s3",
			location: "bucket",
			path:     "segment1/segment2/",
			expected: "s3://bucket/segment1/segment2/",
		},
		{
			name:     "trailing slash with empty path segments",
			scheme:   "s3",
			location: "bucket",
			path:     "path//to///dir/",
			expected: "s3://bucket/path/to/dir/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := uri.Join(test.scheme, test.location, test.path)
			if result != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, result)
			}
		})
	}
}
