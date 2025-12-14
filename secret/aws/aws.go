package aws

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/firetiger-oss/storage/secret"
)

// Manager implements secret.Manager for AWS Secrets Manager
type Manager struct {
	client *secretsmanager.Client
	region string
}

// NewManager creates a new AWS Secrets Manager manager
func NewManager(ctx context.Context, region string) (*Manager, error) {
	if region == "" {
		return nil, fmt.Errorf("AWS region is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return &Manager{
		client: secretsmanager.NewFromConfig(cfg),
		region: region,
	}, nil
}

func (m *Manager) CreateSecret(ctx context.Context, name string, value secret.Value, options ...secret.CreateOption) (secret.Info, error) {
	opts := secret.NewCreateOptions(options...)

	// Build tags
	var awsTags []types.Tag
	for key, val := range opts.Tags() {
		awsTags = append(awsTags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(val),
		})
	}

	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretBinary: value,
		Tags:         awsTags,
	}

	if desc := opts.Description(); desc != "" {
		input.Description = aws.String(desc)
	}

	result, err := m.client.CreateSecret(ctx, input)
	if err != nil {
		return secret.Info{}, convertError(err)
	}

	return secret.Info{
		Name:      name,
		Version:   aws.ToString(result.VersionId),
		CreatedAt: time.Now(),
		Tags:      opts.Tags(),
	}, nil
}

func (m *Manager) GetSecret(ctx context.Context, name string, options ...secret.GetOption) (secret.Value, secret.Info, error) {
	opts := secret.NewGetOptions(options...)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	if version := opts.Version(); version != "" {
		input.VersionId = aws.String(version)
	}

	result, err := m.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, secret.Info{}, convertError(err)
	}

	var value secret.Value
	if result.SecretBinary != nil {
		value = result.SecretBinary
	} else if result.SecretString != nil {
		value = secret.Value(*result.SecretString)
	}

	info := secret.Info{
		Name:      name,
		Version:   aws.ToString(result.VersionId),
		CreatedAt: aws.ToTime(result.CreatedDate),
	}

	return value, info, nil
}

func (m *Manager) UpdateSecret(ctx context.Context, name string, value secret.Value, options ...secret.UpdateOption) (secret.Info, error) {
	opts := secret.NewUpdateOptions(options...)

	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretBinary: value,
	}

	result, err := m.client.PutSecretValue(ctx, input)
	if err != nil {
		return secret.Info{}, convertError(err)
	}

	info := secret.Info{
		Name:    name,
		Version: aws.ToString(result.VersionId),
	}

	// If description is provided, update it separately
	if desc := opts.Description(); desc != "" {
		_, err := m.client.UpdateSecret(ctx, &secretsmanager.UpdateSecretInput{
			SecretId:    aws.String(name),
			Description: aws.String(desc),
		})
		if err != nil {
			return info, convertError(err)
		}
	}

	return info, nil
}

func (m *Manager) DeleteSecret(ctx context.Context, name string) error {
	_, err := m.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(name),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	return convertError(err)
}

func (m *Manager) ListSecrets(ctx context.Context, options ...secret.ListOption) iter.Seq2[secret.Secret, error] {
	opts := secret.NewListOptions(options...)

	return func(yield func(secret.Secret, error) bool) {
		input := &secretsmanager.ListSecretsInput{}

		if maxResults := opts.MaxResults(); maxResults > 0 {
			input.MaxResults = aws.Int32(int32(maxResults))
		}

		// Build filters for tags
		if len(opts.Tags()) > 0 {
			for key, value := range opts.Tags() {
				input.Filters = append(input.Filters, types.Filter{
					Key:    types.FilterNameStringTypeTagKey,
					Values: []string{key},
				})
				input.Filters = append(input.Filters, types.Filter{
					Key:    types.FilterNameStringTypeTagValue,
					Values: []string{value},
				})
			}
		}

		// Add name prefix filter if specified
		if prefix := opts.NamePrefix(); prefix != "" {
			input.Filters = append(input.Filters, types.Filter{
				Key:    types.FilterNameStringTypeName,
				Values: []string{prefix},
			})
		}

		paginator := secretsmanager.NewListSecretsPaginator(m.client, input)

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				yield(secret.Secret{}, convertError(err))
				return
			}

			for _, s := range page.SecretList {
				// Convert AWS tags to map
				tags := make(map[string]string)
				for _, tag := range s.Tags {
					tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}

				sec := secret.Secret{
					Name:      aws.ToString(s.Name),
					CreatedAt: aws.ToTime(s.CreatedDate),
					UpdatedAt: aws.ToTime(s.LastChangedDate),
					Tags:      tags,
				}

				if !yield(sec, nil) {
					return
				}
			}
		}
	}
}

func (m *Manager) ListSecretVersions(ctx context.Context, name string, options ...secret.ListVersionOption) iter.Seq2[secret.Version, error] {
	opts := secret.NewListVersionOptions(options...)

	return func(yield func(secret.Version, error) bool) {
		input := &secretsmanager.ListSecretVersionIdsInput{
			SecretId: aws.String(name),
		}

		if maxResults := opts.MaxResults(); maxResults > 0 {
			input.MaxResults = aws.Int32(int32(maxResults))
		}

		paginator := secretsmanager.NewListSecretVersionIdsPaginator(m.client, input)

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				yield(secret.Version{}, convertError(err))
				return
			}

			for _, versionEntry := range page.Versions {
				// Determine state based on version stages
				state := secret.VersionStateDisabled
				for _, stage := range versionEntry.VersionStages {
					if stage == "AWSCURRENT" {
						state = secret.VersionStateEnabled
						break
					}
				}

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
					ID:        aws.ToString(versionEntry.VersionId),
					State:     state,
					CreatedAt: aws.ToTime(versionEntry.CreatedDate),
				}

				if !yield(version, nil) {
					return
				}
			}
		}
	}
}

func (m *Manager) GetSecretVersion(ctx context.Context, name string, version string) (secret.Value, secret.Info, error) {
	result, err := m.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:  aws.String(name),
		VersionId: aws.String(version),
	})
	if err != nil {
		return nil, secret.Info{}, convertError(err)
	}

	var value secret.Value
	if result.SecretBinary != nil {
		value = result.SecretBinary
	} else if result.SecretString != nil {
		value = secret.Value(*result.SecretString)
	}

	info := secret.Info{
		Name:      name,
		Version:   version,
		CreatedAt: aws.ToTime(result.CreatedDate),
	}

	return value, info, nil
}

func (m *Manager) DestroySecretVersion(ctx context.Context, name string, version string) error {
	// AWS doesn't support destroying individual versions
	// Versions are automatically removed after the secret is deleted
	return fmt.Errorf("destroying individual versions is not supported by AWS Secrets Manager: %w", secret.ErrVersionNotFound)
}
