package aws

import (
	"context"
	"strings"
	"testing"
)

func TestParseRegion(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		wantRegion string
		wantErr    bool
	}{
		{
			name:       "valid ARN",
			identifier: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret",
			wantRegion: "us-east-1",
			wantErr:    false,
		},
		{
			name:       "different region",
			identifier: "arn:aws:secretsmanager:eu-west-1:123456789012:secret:my-secret",
			wantRegion: "eu-west-1",
			wantErr:    false,
		},
		{
			name:       "invalid ARN",
			identifier: "invalid",
			wantRegion: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRegion(tt.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRegion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantRegion {
				t.Errorf("parseRegion() = %v, want %v", got, tt.wantRegion)
			}
		})
	}
}

func TestRegistry_ParseSecret(t *testing.T) {
	reg := &registry{}

	tests := []struct {
		name           string
		identifier     string
		wantManagerID  string
		wantSecretName string
		wantErr        bool
	}{
		{
			name:           "valid ARN",
			identifier:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
			wantManagerID:  "arn:aws:secretsmanager:us-east-1:123456789012",
			wantSecretName: "my-secret-AbCdEf",
			wantErr:        false,
		},
		{
			name:           "ARN without suffix",
			identifier:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret",
			wantManagerID:  "arn:aws:secretsmanager:us-east-1:123456789012",
			wantSecretName: "my-secret",
			wantErr:        false,
		},
		{
			name:           "secret with slashes",
			identifier:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:app/database/password",
			wantManagerID:  "arn:aws:secretsmanager:us-east-1:123456789012",
			wantSecretName: "app/database/password",
			wantErr:        false,
		},
		{
			name:           "invalid ARN",
			identifier:     "invalid",
			wantManagerID:  "",
			wantSecretName: "",
			wantErr:        true,
		},
		{
			name:           "incomplete ARN",
			identifier:     "arn:aws:secretsmanager:us-east-1",
			wantManagerID:  "",
			wantSecretName: "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotManagerID, gotSecretName, err := reg.ParseSecret(tt.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotManagerID != tt.wantManagerID {
				t.Errorf("ParseSecret() managerID = %v, want %v", gotManagerID, tt.wantManagerID)
			}
			if gotSecretName != tt.wantSecretName {
				t.Errorf("ParseSecret() secretName = %v, want %v", gotSecretName, tt.wantSecretName)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	ctx := context.Background()

	t.Run("empty region", func(t *testing.T) {
		_, err := NewManager(ctx, "")
		if err == nil {
			t.Error("expected error for empty region")
		}
		if !strings.Contains(err.Error(), "region is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Integration test - requires AWS credentials
	t.Run("valid region", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		mgr, err := NewManager(ctx, "us-east-1")
		if err != nil {
			t.Skipf("skipping: unable to create manager (likely missing AWS credentials): %v", err)
		}

		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})
}
