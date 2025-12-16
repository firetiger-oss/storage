package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/firetiger-oss/storage/secret"
)

// arnPattern extracts components from an ARN or partial ARN.
// Format: arn:PARTITION:SERVICE:REGION:ACCOUNT:RESOURCE
// Groups: 1=partition, 2=service, 3=region, 4=account, 5=resource (optional)
var arnPattern = regexp.MustCompile(`^arn:([^:]+):([^:]+):([^:]*):([^:]*):?(.*)$`)

type registry struct{}

func init() {
	// Register AWS backend with pattern that matches Secrets Manager ARNs
	// Format: arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME[-SUFFIX]
	secret.Register(`^arn:aws:secretsmanager:`, &registry{})
}

func (r *registry) LoadManager(ctx context.Context, identifier string) (secret.Manager, error) {
	matches := arnPattern.FindStringSubmatch(identifier)
	if matches == nil {
		return nil, fmt.Errorf("invalid ARN: cannot parse %q", identifier)
	}
	region := matches[3]

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return NewManagerFromConfig(cfg), nil
}

func (r *registry) ParseSecret(identifier string) (managerID, secretName string, err error) {
	matches := arnPattern.FindStringSubmatch(identifier)
	if matches == nil {
		return "", "", fmt.Errorf("invalid AWS Secrets Manager ARN: cannot parse %q", identifier)
	}

	partition := matches[1]
	service := matches[2]
	region := matches[3]
	account := matches[4]
	resource := matches[5]

	if service != "secretsmanager" {
		return "", "", fmt.Errorf("invalid AWS Secrets Manager ARN: expected service 'secretsmanager', got %q", service)
	}

	// Resource format is "secret:NAME[-SUFFIX]"
	// Extract the secret name from after "secret:"
	if !strings.HasPrefix(resource, "secret:") {
		return "", "", fmt.Errorf("invalid AWS Secrets Manager ARN: expected resource type 'secret:', got %q", resource)
	}
	secretName = strings.TrimPrefix(resource, "secret:")

	// Manager ID is the ARN prefix without the resource
	managerID = "arn:" + partition + ":" + service + ":" + region + ":" + account

	return managerID, secretName, nil
}
