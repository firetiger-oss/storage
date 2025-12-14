package gcp

import (
	"context"
	"strings"
	"testing"
)

func TestParseProjectID(t *testing.T) {
	tests := []struct {
		name          string
		identifier    string
		wantProjectID string
		wantErr       bool
	}{
		{
			name:          "valid resource name",
			identifier:    "projects/my-project/secrets/my-secret",
			wantProjectID: "my-project",
			wantErr:       false,
		},
		{
			name:          "just project",
			identifier:    "projects/my-project",
			wantProjectID: "my-project",
			wantErr:       false,
		},
		{
			name:          "invalid format",
			identifier:    "invalid",
			wantProjectID: "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProjectID(tt.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseProjectID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantProjectID {
				t.Errorf("parseProjectID() = %v, want %v", got, tt.wantProjectID)
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
			name:           "valid resource name",
			identifier:     "projects/my-project/secrets/my-secret",
			wantManagerID:  "projects/my-project",
			wantSecretName: "my-secret",
			wantErr:        false,
		},
		{
			name:           "resource name with version",
			identifier:     "projects/my-project/secrets/my-secret/versions/1",
			wantManagerID:  "projects/my-project",
			wantSecretName: "my-secret",
			wantErr:        false,
		},
		{
			name:           "invalid resource name",
			identifier:     "invalid",
			wantManagerID:  "",
			wantSecretName: "",
			wantErr:        true,
		},
		{
			name:           "incomplete resource name",
			identifier:     "projects/my-project",
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

	t.Run("empty project ID", func(t *testing.T) {
		_, err := NewManager(ctx, "")
		if err == nil {
			t.Error("expected error for empty project ID")
		}
		if !strings.Contains(err.Error(), "project ID is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Integration test - requires GCP credentials
	t.Run("valid project ID", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		mgr, err := NewManager(ctx, "test-project")
		if err != nil {
			t.Skipf("skipping: unable to create manager (likely missing GCP credentials): %v", err)
		}

		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})
}

func TestExtractSecretName(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		want         string
	}{
		{
			name:         "full secret path",
			resourceName: "projects/my-project/secrets/my-secret",
			want:         "my-secret",
		},
		{
			name:         "version path",
			resourceName: "projects/my-project/secrets/my-secret/versions/1",
			want:         "my-secret",
		},
		{
			name:         "just name",
			resourceName: "my-secret",
			want:         "my-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSecretName(tt.resourceName); got != tt.want {
				t.Errorf("extractSecretName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractVersionID(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		want         string
	}{
		{
			name:         "version path",
			resourceName: "projects/my-project/secrets/my-secret/versions/1",
			want:         "1",
		},
		{
			name:         "latest version",
			resourceName: "projects/my-project/secrets/my-secret/versions/latest",
			want:         "latest",
		},
		{
			name:         "no version",
			resourceName: "projects/my-project/secrets/my-secret",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractVersionID(tt.resourceName); got != tt.want {
				t.Errorf("extractVersionID() = %v, want %v", got, tt.want)
			}
		})
	}
}
