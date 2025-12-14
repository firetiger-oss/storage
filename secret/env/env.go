// Package env provides a read-only secret manager implementation backed by
// environment variables. This is intended for local development and testing.
//
// The env backend maps environment variable names to secret names.
// All write operations (Create, Update, Delete, Rotate) return ErrReadOnly.
//
// Example usage:
//
//	manager, err := secret.LoadManager(ctx, "secret://env")
//	if err != nil {
//		return err
//	}
//
//	// Read an environment variable as a secret
//	value, info, err := manager.Get(ctx, "DATABASE_URL")
//	if err != nil {
//		return err
//	}
package env

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"strings"

	"github.com/firetiger-oss/storage/secret"
)

var (
	errLocationNotSupported = errors.New("env backend does not support location specifiers")
)

func init() {
	secret.Register("env", NewRegistry())
}

// NewRegistry creates a registry for the environment variables backend.
func NewRegistry() secret.Registry {
	return secret.RegistryFunc(func(ctx context.Context, location string) (secret.Manager, error) {
		// Parse the location to check if it's empty or just "secret://env"
		if location != "" && location != "secret://env" && location != "env://" {
			return nil, errLocationNotSupported
		}
		return NewManager(), nil
	})
}

// Manager is a read-only secret manager backed by environment variables.
type Manager struct{}

// NewManager creates a new environment variables secret manager.
func NewManager() *Manager {
	return &Manager{}
}

// Create returns ErrReadOnly since the env backend is read-only.
func (m *Manager) CreateSecret(ctx context.Context, name string, value secret.Value, options ...secret.CreateOption) (secret.Info, error) {
	return secret.Info{}, secret.ErrReadOnly
}

// Get retrieves an environment variable as a secret.
// Returns ErrNotFound if the environment variable is not set.
func (m *Manager) GetSecret(ctx context.Context, name string, options ...secret.GetOption) (secret.Value, secret.Info, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, secret.Info{}, err
	}

	value, ok := os.LookupEnv(name)
	if !ok {
		return nil, secret.Info{}, fmt.Errorf("%s: %w", name, secret.ErrNotFound)
	}

	return secret.Value(value), secret.Info{
		Name: name,
		// No version, tags, timestamps, or description available for env vars
	}, nil
}

// Update returns ErrReadOnly since the env backend is read-only.
func (m *Manager) UpdateSecret(ctx context.Context, name string, value secret.Value, options ...secret.UpdateOption) (secret.Info, error) {
	return secret.Info{}, secret.ErrReadOnly
}

// Delete returns ErrReadOnly since the env backend is read-only.
func (m *Manager) DeleteSecret(ctx context.Context, name string) error {
	return secret.ErrReadOnly
}

// List returns an iterator of environment variables matching the given options.
// Only NamePrefix filtering is supported; tag filtering is ignored.
func (m *Manager) ListSecrets(ctx context.Context, options ...secret.ListOption) iter.Seq2[secret.Secret, error] {
	return func(yield func(secret.Secret, error) bool) {
		if err := context.Cause(ctx); err != nil {
			yield(secret.Secret{}, err)
			return
		}

		opts := secret.NewListOptions(options...)
		prefix := opts.NamePrefix()
		maxResults := opts.MaxResults()
		count := 0

		for _, envVar := range os.Environ() {
			// Parse the environment variable
			key, _, found := strings.Cut(envVar, "=")
			if !found {
				continue
			}

			// Apply prefix filter
			if prefix != "" && !strings.HasPrefix(key, prefix) {
				continue
			}

			// Check max results
			if maxResults > 0 && count >= maxResults {
				return
			}

			if !yield(secret.Secret{Name: key}, nil) {
				return
			}
			count++
		}
	}
}

// ListVersions returns an empty iterator since the env backend doesn't support versioning.
func (m *Manager) ListSecretVersions(ctx context.Context, name string, options ...secret.ListVersionOption) iter.Seq2[secret.Version, error] {
	return func(yield func(secret.Version, error) bool) {
		// Environment variables don't have versions
		// Return empty iterator
	}
}

// GetVersion returns ErrVersionNotFound since the env backend doesn't support versioning.
func (m *Manager) GetSecretVersion(ctx context.Context, name string, version string) (secret.Value, secret.Info, error) {
	return nil, secret.Info{}, secret.ErrVersionNotFound
}

// DestroyVersion returns ErrVersionNotFound since the env backend doesn't support versioning.
func (m *Manager) DestroySecretVersion(ctx context.Context, name string, version string) error {
	return secret.ErrVersionNotFound
}
