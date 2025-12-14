package aws

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/firetiger-oss/storage/secret"
)

// convertError converts AWS Secrets Manager errors to standard secret errors
func convertError(err error) error {
	if err == nil {
		return nil
	}

	// Check for AWS-specific error types
	var notFound *types.ResourceNotFoundException
	if errors.As(err, &notFound) {
		return fmt.Errorf("%w: %v", secret.ErrNotFound, err)
	}

	var alreadyExists *types.ResourceExistsException
	if errors.As(err, &alreadyExists) {
		return fmt.Errorf("%w: %v", secret.ErrAlreadyExists, err)
	}

	var invalidRequest *types.InvalidRequestException
	if errors.As(err, &invalidRequest) {
		return fmt.Errorf("%w: %v", secret.ErrInvalidName, err)
	}

	// Return the original error if no conversion applies
	return err
}
