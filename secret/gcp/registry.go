package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/firetiger-oss/storage/secret"
)

type registry struct{}

func init() {
	// Register GCP backend with pattern that matches Secret Manager resource names
	// Format: projects/PROJECT_ID/secrets/SECRET_NAME[/versions/VERSION_ID]
	secret.Register(`^projects/[^/]+/(secrets|locations)`, &registry{})
}

func (r *registry) LoadManager(ctx context.Context, identifier string) (secret.Manager, error) {
	// Parse the resource name to extract project ID
	projectID, err := parseProjectID(identifier)
	if err != nil {
		return nil, err
	}

	return NewManager(ctx, projectID)
}

func (r *registry) ParseSecret(identifier string) (managerID, secretName string, err error) {
	if !strings.HasPrefix(identifier, "projects/") {
		return "", "", fmt.Errorf("invalid GCP Secret Manager resource name: %s", identifier)
	}

	// Parse resource name: projects/PROJECT_ID/secrets/SECRET_NAME[/versions/VERSION_ID]
	parts := strings.Split(identifier, "/")
	if len(parts) < 4 {
		return "", "", fmt.Errorf("invalid GCP Secret Manager resource name format: %s", identifier)
	}

	if parts[0] != "projects" {
		return "", "", fmt.Errorf("invalid GCP resource name, must start with 'projects/': %s", identifier)
	}

	projectID := parts[1]

	// Check if this is a secret reference
	if parts[2] != "secrets" {
		return "", "", fmt.Errorf("invalid GCP resource name, expected 'secrets': %s", identifier)
	}

	// Manager ID is projects/PROJECT_ID
	managerID = fmt.Sprintf("projects/%s", projectID)

	// Secret name is just the name part (not the full path)
	secretName = parts[3]

	return managerID, secretName, nil
}

// parseProjectID extracts the project ID from a GCP resource name or identifier
func parseProjectID(identifier string) (string, error) {
	// For resource names: projects/PROJECT_ID/...
	if strings.HasPrefix(identifier, "projects/") {
		parts := strings.Split(identifier, "/")
		if len(parts) >= 2 {
			return parts[1], nil
		}
		return "", fmt.Errorf("invalid resource name format: %s", identifier)
	}

	return "", fmt.Errorf("unsupported identifier format: %s", identifier)
}
