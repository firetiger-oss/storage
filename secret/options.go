package secret

import (
	"iter"
	"maps"
	"slices"
	"time"
)

// CreateOption is a functional option for the Create method.
type CreateOption func(*CreateOptions)

// CreateOptions contains options for creating a secret.
type CreateOptions struct {
	tags        map[string]string
	description string
	expiresAt   time.Time
}

// Tags returns the tags set on the options.
func (o *CreateOptions) Tags() map[string]string {
	return o.tags
}

// Description returns the description set on the options.
func (o *CreateOptions) Description() string {
	return o.description
}

// ExpiresAt returns the expiration time set on the options.
func (o *CreateOptions) ExpiresAt() time.Time {
	return o.expiresAt
}

// NewCreateOptions creates CreateOptions from an iterator of CreateOption functions.
func NewCreateOptions(options ...CreateOption) *CreateOptions {
	return NewOptions[CreateOptions](slices.Values(options))
}

// Tags sets multiple tags on the secret.
func Tags(tags map[string]string) CreateOption {
	return func(o *CreateOptions) {
		if o.tags == nil {
			o.tags = make(map[string]string)
		}
		maps.Copy(o.tags, tags)
	}
}

// Tag sets a single tag on the secret.
func Tag(key, value string) CreateOption {
	return func(o *CreateOptions) {
		if o.tags == nil {
			o.tags = make(map[string]string)
		}
		o.tags[key] = value
	}
}

// Description sets a description for the secret.
func Description(desc string) CreateOption {
	return func(o *CreateOptions) {
		o.description = desc
	}
}

// ExpiresAt sets an expiration time for the secret.
// Not all backends support expiration.
func ExpiresAt(t time.Time) CreateOption {
	return func(o *CreateOptions) {
		o.expiresAt = t
	}
}

// GetOption is a functional option for the Get method.
type GetOption func(*GetOptions)

// GetOptions contains options for getting a secret.
type GetOptions struct {
	version string
}

// Version returns the version set on the options.
func (o *GetOptions) Version() string {
	return o.version
}

// NewGetOptions creates GetOptions from an iterator of GetOption functions.
func NewGetOptions(options ...GetOption) *GetOptions {
	return NewOptions[GetOptions](slices.Values(options))
}

// WithVersion specifies which version of the secret to retrieve.
// If not specified, the latest version is returned.
func WithVersion(v string) GetOption {
	return func(o *GetOptions) {
		o.version = v
	}
}

// UpdateOption is a functional option for the Update method.
type UpdateOption func(*UpdateOptions)

// UpdateOptions contains options for updating a secret.
type UpdateOptions struct {
	description string
}

// Description returns the description set on the options.
func (o *UpdateOptions) Description() string {
	return o.description
}

// NewUpdateOptions creates UpdateOptions from an iterator of UpdateOption functions.
func NewUpdateOptions(options ...UpdateOption) *UpdateOptions {
	return NewOptions[UpdateOptions](slices.Values(options))
}

// UpdateDescription is an UpdateOption that updates the secret's description.
func UpdateDescription(desc string) UpdateOption {
	return func(o *UpdateOptions) {
		o.description = desc
	}
}

// ListOption is a functional option for the List method.
type ListOption func(*ListOptions)

// ListOptions contains options for listing secrets.
type ListOptions struct {
	namePrefix string
	tags       map[string]string
	maxResults int
}

// NamePrefix returns the name prefix filter.
func (o *ListOptions) NamePrefix() string {
	return o.namePrefix
}

// Tags returns the tag filters.
func (o *ListOptions) Tags() map[string]string {
	return o.tags
}

// MaxResults returns the maximum number of results to return.
func (o *ListOptions) MaxResults() int {
	return o.maxResults
}

// NewListOptions creates ListOptions from an iterator of ListOption functions.
func NewListOptions(options ...ListOption) *ListOptions {
	return NewOptions[ListOptions](slices.Values(options))
}

// NamePrefix filters secrets by name prefix.
func NamePrefix(prefix string) ListOption {
	return func(o *ListOptions) {
		o.namePrefix = prefix
	}
}

// FilterByTag filters secrets by a single tag key-value pair.
func FilterByTag(key, value string) ListOption {
	return func(o *ListOptions) {
		if o.tags == nil {
			o.tags = make(map[string]string)
		}
		o.tags[key] = value
	}
}

// FilterByTags filters secrets by multiple tags.
// Secrets must match all specified tags.
func FilterByTags(tags map[string]string) ListOption {
	return func(o *ListOptions) {
		if o.tags == nil {
			o.tags = make(map[string]string)
		}
		maps.Copy(o.tags, tags)
	}
}

// MaxResults limits the number of results returned.
func MaxResults(n int) ListOption {
	return func(o *ListOptions) {
		o.maxResults = n
	}
}

// ListVersionOption is a functional option for the ListVersions method.
type ListVersionOption func(*ListVersionOptions)

// ListVersionOptions contains options for listing secret versions.
type ListVersionOptions struct {
	maxResults int
	states     []VersionState
}

// MaxResults returns the maximum number of results to return.
func (o *ListVersionOptions) MaxResults() int {
	return o.maxResults
}

// States returns the version state filters.
func (o *ListVersionOptions) States() []VersionState {
	return o.states
}

// NewListVersionOptions creates ListVersionOptions from an iterator of ListVersionOption functions.
func NewListVersionOptions(options ...ListVersionOption) *ListVersionOptions {
	return NewOptions[ListVersionOptions](slices.Values(options))
}

// MaxVersions limits the number of versions returned.
func MaxVersions(n int) ListVersionOption {
	return func(o *ListVersionOptions) {
		o.maxResults = n
	}
}

// FilterByState filters versions by their state.
// Versions must match one of the specified states.
func FilterByState(states ...VersionState) ListVersionOption {
	return func(o *ListVersionOptions) {
		o.states = append(o.states, states...)
	}
}

// NewOptions is a generic helper for creating options from an iterator.
// This follows the same pattern as the storage package.
func NewOptions[Options any, Option ~func(*Options)](options iter.Seq[Option]) *Options {
	opts := new(Options)
	for option := range options {
		option(opts)
	}
	return opts
}
