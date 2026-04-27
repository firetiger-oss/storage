package gcp

import (
	"fmt"

	"github.com/firetiger-oss/tigerblock/secret"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// convertError converts GCP Secret Manager errors to standard secret errors
func convertError(err error) error {
	if err == nil {
		return nil
	}

	// Check for gRPC status codes
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%w: %v", secret.ErrNotFound, err)
	case codes.AlreadyExists:
		return fmt.Errorf("%w: %v", secret.ErrAlreadyExists, err)
	case codes.InvalidArgument:
		return fmt.Errorf("%w: %v", secret.ErrInvalidName, err)
	case codes.PermissionDenied, codes.Unauthenticated:
		return err // Return permission errors as-is
	default:
		return err
	}
}
