package http_test

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/firetiger-oss/storage"
	storagehttp "github.com/firetiger-oss/storage/http"
	"github.com/firetiger-oss/storage/memory"
	s3storage "github.com/firetiger-oss/storage/s3"
	storagetest "github.com/firetiger-oss/storage/test"
)

func TestHTTPStorage(t *testing.T) {
	tests := []struct {
		scenario string
		options  []storagehttp.BucketOption
	}{
		{
			scenario: "default",
			options:  []storagehttp.BucketOption{},
		},
		{
			scenario: "list-type=1",
			options: []storagehttp.BucketOption{
				storagehttp.WithListType("1"),
			},
		},
		{
			scenario: "list-type=2",
			options: []storagehttp.BucketOption{
				storagehttp.WithListType("2"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			storagetest.TestStorage(t, func(t *testing.T) (storage.Bucket, error) {
				l, err := net.Listen("tcp", ":0")
				if err != nil {
					return nil, err
				}

				location := "http://" + l.Addr().String()

				s := &http.Server{
					Handler: storagehttp.BucketHandler(new(memory.Bucket),
						storagehttp.WithLocation(location),
						storagehttp.WithMaxKeys(1),
					),
				}

				go s.Serve(l)

				t.Cleanup(func() {
					s.Close()
					l.Close()
				})

				return storagehttp.NewRegistry("http", test.options...).LoadBucket(t.Context(), location)
			})
		})
	}
}

// TestHTTPStorageWithS3Client tests the HTTP storage implementation
// using the S3 client as a client, to ensure S3 compatibility.
func TestHTTPStorageWithS3Client(t *testing.T) {
	storagetest.TestStorage(t, func(t *testing.T) (storage.Bucket, error) {
		// We have to strip the "/testbucket" prefix from the URL because the
		// S3 client uses path-style due to setting the endpoint resolver with
		// an immutable hostname.
		server := httptest.NewServer(
			storagehttp.StripBucketNamePrefix("testbucket",
				storagehttp.BucketHandler(new(memory.Bucket)),
			),
		)

		t.Cleanup(func() {
			server.Close()
		})

		s3Config, err := config.LoadDefaultConfig(t.Context())
		if err != nil {
			return nil, err
		}

		s3Client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
			o.Region = "us-east-1"
			o.Credentials = aws.AnonymousCredentials{}
			o.BaseEndpoint = aws.String(server.URL)
			o.UsePathStyle = true
			o.HTTPClient = &http.Client{
				Transport: &debugTransport{
					transport: http.DefaultTransport,
					t:         t,
				},
			}
		})
		return s3storage.NewBucket(s3Client, "testbucket"), nil
	})
}

type debugTransport struct {
	transport http.RoundTripper
	t         testing.TB
}

func (debug *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b, _ := httputil.DumpRequestOut(req, false)
	debug.t.Log(string(b))

	res, err := debug.transport.RoundTrip(req)
	if err != nil {
		debug.t.Logf("error make http request: %v\n", err)
		return nil, err
	}

	b, _ = httputil.DumpResponse(res, false)
	debug.t.Log(string(b))

	res.Body = &debugReadCloser{body: res.Body, t: debug.t}
	return res, nil

}

type debugReadCloser struct {
	body io.ReadCloser
	read int64
	t    testing.TB
}

func (debug *debugReadCloser) Read(p []byte) (int, error) {
	n, err := debug.body.Read(p)
	debug.read += int64(n)
	if err != nil && err != io.EOF {
		debug.t.Logf("error reading the response body after reading %d bytes: %v\n", debug.read, err)
	}
	return n, err
}

func (debug *debugReadCloser) Close() error {
	err := debug.body.Close()
	if err != nil {
		debug.t.Logf("error closing the response body after reading %d bytes: %v\n", debug.read, err)
	}
	return err
}
