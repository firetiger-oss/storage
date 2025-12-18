package gcp

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/firetiger-oss/storage/secret"
)

type registry struct{}

func init() {
	secret.Register("projects/", &registry{})
}

func (r *registry) LoadManager(ctx context.Context, identifier string) (secret.Manager, error) {
	// Parse the resource name to extract project ID
	projectID, err := parseProjectID(identifier)
	if err != nil {
		return nil, err
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create GCP secret manager client: %w", err)
	}

	return NewManagerFromClient(&clientAdapter{client: client}, projectID), nil
}

func (r *registry) ParseSecret(identifier string) (managerID, secretName, version string, err error) {
	// Parse resource name: projects/PROJECT_ID/secrets/SECRET_NAME[/versions/VERSION_ID]
	prefix, rest, ok := strings.Cut(identifier, "/")
	if !ok || prefix != "projects" {
		return "", "", "", fmt.Errorf("invalid GCP Secret Manager resource name: %s", identifier)
	}

	projectID, rest, _ := strings.Cut(rest, "/")
	if projectID == "" {
		return "", "", "", fmt.Errorf("invalid GCP Secret Manager resource name format: %s", identifier)
	}

	secretsLiteral, rest, ok := strings.Cut(rest, "/")
	if !ok || (secretsLiteral != "secrets" && secretsLiteral != "locations") {
		// No /secrets/ found - this is just a manager ID (projects/PROJECT_ID)
		return "projects/" + projectID, "", "", nil
	}

	// Handle locations/LOCATION/secrets/NAME format
	if secretsLiteral == "locations" {
		_, rest, ok = strings.Cut(rest, "/") // skip location ID
		if !ok {
			return "projects/" + projectID, "", "", nil
		}
		secretsLiteral, rest, ok = strings.Cut(rest, "/")
		if !ok || secretsLiteral != "secrets" {
			return "projects/" + projectID, "", "", nil
		}
	}

	secretName, rest, _ = strings.Cut(rest, "/")
	if secretName == "" {
		return "projects/" + projectID, "", "", nil
	}

	// Check for /versions/VERSION_ID
	if versionsLiteral, v, ok := strings.Cut(rest, "/"); ok && versionsLiteral == "versions" {
		version = v
	}

	return "projects/" + projectID, secretName, version, nil
}

// parseProjectID extracts the project ID from a GCP resource name or identifier
func parseProjectID(identifier string) (string, error) {
	// For resource names: projects/PROJECT_ID/...
	rest, ok := strings.CutPrefix(identifier, "projects/")
	if !ok {
		return "", fmt.Errorf("unsupported identifier format: %s", identifier)
	}
	projectID, _, _ := strings.Cut(rest, "/")
	if projectID == "" {
		return "", fmt.Errorf("invalid resource name format: %s", identifier)
	}
	return projectID, nil
}
