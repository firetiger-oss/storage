package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/firetiger-oss/storage/secret"
)

type registry struct{}

func init() {
	// Register AWS backend with pattern that matches Secrets Manager ARNs
	// Format: arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME[-SUFFIX]
	secret.Register(`^arn:aws:secretsmanager:`, &registry{})
}

func (r *registry) LoadManager(ctx context.Context, identifier string) (secret.Manager, error) {
	// Parse the ARN to extract region
	region, err := parseRegion(identifier)
	if err != nil {
		return nil, err
	}

	return NewManager(ctx, region)
}

func (r *registry) ParseSecret(identifier string) (managerID, secretName string, err error) {
	if !strings.HasPrefix(identifier, "arn:aws:secretsmanager:") {
		return "", "", fmt.Errorf("invalid AWS Secrets Manager ARN: %s", identifier)
	}

	// Parse ARN: arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME[-SUFFIX]
	parts := strings.Split(identifier, ":")
	if len(parts) < 7 {
		return "", "", fmt.Errorf("invalid AWS Secrets Manager ARN format: %s", identifier)
	}

	region := parts[3]
	account := parts[4]

	// Manager ID is the ARN prefix without the secret name
	// arn:aws:secretsmanager:REGION:ACCOUNT
	managerID = fmt.Sprintf("arn:aws:secretsmanager:%s:%s", region, account)

	// Secret name is everything after "secret:"
	// This includes the name and optional suffix
	secretName = strings.Join(parts[6:], ":")

	return managerID, secretName, nil
}

// parseRegion extracts the region from an AWS ARN or identifier
func parseRegion(identifier string) (string, error) {
	// For ARNs: arn:aws:secretsmanager:REGION:ACCOUNT[:...]
	if strings.HasPrefix(identifier, "arn:aws:secretsmanager:") {
		parts := strings.Split(identifier, ":")
		if len(parts) >= 4 {
			return parts[3], nil
		}
		return "", fmt.Errorf("invalid ARN format: %s", identifier)
	}

	return "", fmt.Errorf("unsupported identifier format: %s", identifier)
}
