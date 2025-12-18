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
	arn, name, ok := strings.Cut(identifier, ":secret:")
	if !ok {
		return "", "", fmt.Errorf("invalid AWS Secrets Manager ARN: cannot parse %q", identifier)
	}
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		name = name[:i] // trim stage qualifier
	}
	if i := strings.LastIndexByte(name, '-'); i >= 0 {
		name = name[:i] // trim random suffix
	}
	return arn, name, nil
}
