package env

import (
	"context"
	"fmt"
	"strings"

	"github.com/firetiger-oss/storage/secret"
)

type registry struct{}

func init() {
	// Register env backend with pattern that matches "env" or environment variable names
	// We'll match anything that doesn't look like an ARN or GCP resource name
	secret.Register(`^[A-Z_][A-Z0-9_]*$|^env$`, &registry{})
}

func (r *registry) LoadManager(ctx context.Context, identifier string) (secret.Manager, error) {
	// For env backend, we always return the same global manager
	// The identifier might be "env" or an actual env var name
	return &Manager{}, nil
}

func (r *registry) ParseSecret(identifier string) (managerID, secretName string, err error) {
	// For env backend, the identifier IS the secret name (env var name)
	// The manager is always "env"
	if identifier == "" {
		return "", "", fmt.Errorf("identifier is required")
	}

	// If identifier is just "env", there's no specific secret
	if identifier == "env" {
		return "env", "", nil
	}

	// Check if it looks like an environment variable name
	if !strings.HasPrefix(identifier, "arn:") && !strings.HasPrefix(identifier, "projects/") {
		return "env", identifier, nil
	}

	return "", "", fmt.Errorf("invalid env identifier: %s", identifier)
}
