package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/firetiger-oss/storage/secret"
)

// arnPattern extracts the region from a Secrets Manager ARN.
// Format: arn:PARTITION:secretsmanager:REGION:...
var arnPattern = regexp.MustCompile(`^arn:[^:]+:secretsmanager:([^:]*)`)

type registry struct{}

func init() {
	secret.Register("arn:", &registry{})
}

func (r *registry) LoadManager(ctx context.Context, identifier string) (secret.Manager, error) {
	matches := arnPattern.FindStringSubmatch(identifier)
	if matches == nil {
		return nil, fmt.Errorf("invalid ARN: cannot parse %q", identifier)
	}
	region := matches[1]

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return NewManagerFromConfig(cfg), nil
}

func (r *registry) ParseSecret(identifier string) (managerID, secretName, version string, err error) {
	if !arnPattern.MatchString(identifier) {
		return "", "", "", fmt.Errorf("invalid AWS Secrets Manager ARN: %q", identifier)
	}
	arn, name, ok := strings.Cut(identifier, ":secret:")
	if !ok {
		return identifier, "", "", nil
	}
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		name = name[:i] // strip stage qualifier (AWSCURRENT, AWSPREVIOUS, etc.)
	}
	if i := strings.LastIndexByte(name, '-'); i >= 0 {
		name = name[:i] // strip random suffix
	}
	return arn, name, "", nil
}
