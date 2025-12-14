package gcp

import (
	"context"
	"fmt"
	"iter"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/firetiger-oss/storage/secret"
	"google.golang.org/api/iterator"
)

// Manager implements secret.Manager for GCP Secret Manager
type Manager struct {
	client      *secretmanager.Client
	projectPath string
	projectID   string
}

// NewManager creates a new GCP Secret Manager manager
func NewManager(ctx context.Context, projectID string) (*Manager, error) {
	if projectID == "" {
		return nil, fmt.Errorf("GCP project ID is required")
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create GCP secret manager client: %w", err)
	}

	return &Manager{
		client:      client,
		projectID:   projectID,
		projectPath: fmt.Sprintf("projects/%s", projectID),
	}, nil
}

func (m *Manager) CreateSecret(ctx context.Context, name string, value secret.Value, options ...secret.CreateOption) (secret.Info, error) {
	opts := secret.NewCreateOptions(options...)

	// Build labels (GCP's version of tags)
	labels := make(map[string]string)
	for key, val := range opts.Tags() {
		labels[key] = val
	}

	// Create the secret
	secretReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   m.projectPath,
		SecretId: name,
		Secret: &secretmanagerpb.Secret{
			Labels: labels,
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	sec, err := m.client.CreateSecret(ctx, secretReq)
	if err != nil {
		return secret.Info{}, convertError(err)
	}

	// Add the first version with the value
	versionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: sec.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: value,
		},
	}

	version, err := m.client.AddSecretVersion(ctx, versionReq)
	if err != nil {
		return secret.Info{}, convertError(err)
	}

	return secret.Info{
		Name:      name,
		Version:   extractVersionID(version.Name),
		CreatedAt: sec.CreateTime.AsTime(),
		Tags:      opts.Tags(),
	}, nil
}

func (m *Manager) GetSecret(ctx context.Context, name string, options ...secret.GetOption) (secret.Value, secret.Info, error) {
	opts := secret.NewGetOptions(options...)

	// Build the version path
	var versionPath string
	if version := opts.Version(); version != "" {
		versionPath = fmt.Sprintf("%s/secrets/%s/versions/%s", m.projectPath, name, version)
	} else {
		versionPath = fmt.Sprintf("%s/secrets/%s/versions/latest", m.projectPath, name)
	}

	result, err := m.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: versionPath,
	})
	if err != nil {
		return nil, secret.Info{}, convertError(err)
	}

	info := secret.Info{
		Name:    name,
		Version: extractVersionID(result.Name),
		// GCP AccessSecretVersionResponse doesn't include creation time
		// Would need separate GetSecretVersion call to get it
	}

	return result.Payload.Data, info, nil
}

func (m *Manager) UpdateSecret(ctx context.Context, name string, value secret.Value, options ...secret.UpdateOption) (secret.Info, error) {
	opts := secret.NewUpdateOptions(options...)

	// Add a new version (GCP doesn't have "update", only new versions)
	secretPath := fmt.Sprintf("%s/secrets/%s", m.projectPath, name)

	versionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretPath,
		Payload: &secretmanagerpb.SecretPayload{
			Data: value,
		},
	}

	version, err := m.client.AddSecretVersion(ctx, versionReq)
	if err != nil {
		return secret.Info{}, convertError(err)
	}

	info := secret.Info{
		Name:    name,
		Version: extractVersionID(version.Name),
	}

	// Update description/labels if provided
	if desc := opts.Description(); desc != "" {
		secretPath := fmt.Sprintf("%s/secrets/%s", m.projectPath, name)
		_, err := m.client.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name: secretPath,
				// Note: GCP doesn't have a description field, but we could use a label
				Labels: map[string]string{"description": desc},
			},
		})
		if err != nil {
			return info, convertError(err)
		}
	}

	return info, nil
}

func (m *Manager) DeleteSecret(ctx context.Context, name string) error {
	secretPath := fmt.Sprintf("%s/secrets/%s", m.projectPath, name)

	err := m.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: secretPath,
	})
	return convertError(err)
}

func (m *Manager) ListSecrets(ctx context.Context, options ...secret.ListOption) iter.Seq2[secret.Secret, error] {
	opts := secret.NewListOptions(options...)

	return func(yield func(secret.Secret, error) bool) {
		req := &secretmanagerpb.ListSecretsRequest{
			Parent: m.projectPath,
		}

		if maxResults := opts.MaxResults(); maxResults > 0 {
			req.PageSize = int32(maxResults)
		}

		// GCP uses a filter string for tag filtering
		// Format: labels.KEY=VALUE
		if len(opts.Tags()) > 0 {
			var filters []string
			for key, value := range opts.Tags() {
				filters = append(filters, fmt.Sprintf("labels.%s=%s", key, value))
			}
			req.Filter = fmt.Sprintf("%s", filters[0])
			// Note: GCP filter syntax might need adjustment for multiple tags
		}

		it := m.client.ListSecrets(ctx, req)

		for {
			sec, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				yield(secret.Secret{}, convertError(err))
				return
			}

			// Extract secret name from resource path
			secretName := extractSecretName(sec.Name)

			// Apply name prefix filter (client-side since GCP doesn't support it natively)
			if prefix := opts.NamePrefix(); prefix != "" {
				if !hasPrefix(secretName, prefix) {
					continue
				}
			}

			s := secret.Secret{
				Name:      secretName,
				CreatedAt: sec.CreateTime.AsTime(),
				Tags:      sec.Labels,
			}

			if !yield(s, nil) {
				return
			}
		}
	}
}

func (m *Manager) ListSecretVersions(ctx context.Context, name string, options ...secret.ListVersionOption) iter.Seq2[secret.Version, error] {
	opts := secret.NewListVersionOptions(options...)

	return func(yield func(secret.Version, error) bool) {
		secretPath := fmt.Sprintf("%s/secrets/%s", m.projectPath, name)

		req := &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretPath,
		}

		if maxResults := opts.MaxResults(); maxResults > 0 {
			req.PageSize = int32(maxResults)
		}

		it := m.client.ListSecretVersions(ctx, req)

		for {
			ver, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				yield(secret.Version{}, convertError(err))
				return
			}

			// Map GCP state to secret.VersionState
			state := mapGCPState(ver.State)

			// Filter by state if specified
			if len(opts.States()) > 0 {
				found := false
				for _, s := range opts.States() {
					if s == state {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			version := secret.Version{
				ID:        extractVersionID(ver.Name),
				State:     state,
				CreatedAt: ver.CreateTime.AsTime(),
			}

			if !yield(version, nil) {
				return
			}
		}
	}
}

func (m *Manager) GetSecretVersion(ctx context.Context, name string, version string) (secret.Value, secret.Info, error) {
	versionPath := fmt.Sprintf("%s/secrets/%s/versions/%s", m.projectPath, name, version)

	result, err := m.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: versionPath,
	})
	if err != nil {
		return nil, secret.Info{}, convertError(err)
	}

	info := secret.Info{
		Name:    name,
		Version: version,
		// GCP AccessSecretVersionResponse doesn't include creation time
	}

	return result.Payload.Data, info, nil
}

func (m *Manager) DestroySecretVersion(ctx context.Context, name string, version string) error {
	versionPath := fmt.Sprintf("%s/secrets/%s/versions/%s", m.projectPath, name, version)

	_, err := m.client.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
		Name: versionPath,
	})
	return convertError(err)
}

// Helper functions

// extractSecretName extracts the secret name from a full resource path
// projects/PROJECT_ID/secrets/SECRET_NAME -> SECRET_NAME
func extractSecretName(resourceName string) string {
	parts := splitPath(resourceName)
	if len(parts) >= 4 && parts[2] == "secrets" {
		return parts[3]
	}
	return resourceName
}

// extractVersionID extracts the version ID from a full version resource path
// projects/PROJECT_ID/secrets/SECRET_NAME/versions/VERSION_ID -> VERSION_ID
func extractVersionID(resourceName string) string {
	parts := splitPath(resourceName)
	if len(parts) >= 6 && parts[4] == "versions" {
		return parts[5]
	}
	return ""
}

// splitPath splits a resource path by '/'
func splitPath(path string) []string {
	return splitString(path, "/")
}

// splitString splits a string by a delimiter
func splitString(s, delim string) []string {
	var result []string
	for {
		idx := indexOf(s, delim)
		if idx == -1 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(delim):]
	}
	return result
}

// indexOf returns the index of the first occurrence of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// hasPrefix checks if a string has a given prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// mapGCPState maps GCP SecretVersion state to secret.VersionState
func mapGCPState(state secretmanagerpb.SecretVersion_State) secret.VersionState {
	switch state {
	case secretmanagerpb.SecretVersion_ENABLED:
		return secret.VersionStateEnabled
	case secretmanagerpb.SecretVersion_DISABLED:
		return secret.VersionStateDisabled
	case secretmanagerpb.SecretVersion_DESTROYED:
		return secret.VersionStateDestroyed
	default:
		return secret.VersionStateDisabled
	}
}
