package secret

import "context"

// Provider provides read-only access to secrets by name.
// Manager implements this interface.
type Provider interface {
	// GetSecret retrieves a secret by name.
	// Use WithVersion to retrieve a specific version.
	// Returns ErrNotFound if the secret does not exist.
	// Returns ErrVersionNotFound if the version does not exist.
	GetSecret(ctx context.Context, name string, options ...GetOption) (Value, Info, error)
}

// ProviderFunc is a function adapter for Provider.
type ProviderFunc func(ctx context.Context, name string, options ...GetOption) (Value, Info, error)

// GetSecret implements Provider.
func (f ProviderFunc) GetSecret(ctx context.Context, name string, options ...GetOption) (Value, Info, error) {
	return f(ctx, name, options...)
}
