// Package test provides a comprehensive test suite for secret.Manager implementations.
// The test suite validates all required behaviors across different backends and adapters.
package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/firetiger-oss/storage/secret"
)

// TestManager runs a comprehensive test suite against a secret manager implementation.
// The loadManager function should create a fresh manager instance for each test.
//
// Example usage:
//
//	test.TestManager(t, func(t *testing.T) (secret.Manager, error) {
//		return secret.LoadManager(t.Context(), "secret://env")
//	})
func TestManager(t *testing.T, loadManager func(*testing.T) (secret.Manager, error)) {
	// Test with different adapters to ensure adapters don't break functionality
	adapters := []struct {
		name    string
		adapter secret.Adapter
	}{
		{
			name:    "base",
			adapter: secret.AdapterFunc(func(m secret.Manager) secret.Manager { return m }),
		},
	}

	tests := []struct {
		scenario string
		function func(*testing.T, secret.Manager)
		skipIf   func(secret.Manager) (bool, string)
	}{
		{
			scenario: "creating and retrieving a secret works",
			function: testCreateAndGet,
			skipIf:   skipIfReadOnly,
		},
		{
			scenario: "getting a non-existent secret returns error",
			function: testGetNotExist,
		},
		{
			scenario: "creating a duplicate secret returns error",
			function: testCreateDuplicate,
			skipIf:   skipIfReadOnly,
		},
		{
			scenario: "updating a secret works",
			function: testUpdate,
			skipIf:   skipIfReadOnly,
		},
		{
			scenario: "deleting a secret works",
			function: testDelete,
			skipIf:   skipIfReadOnly,
		},
		{
			scenario: "delete is idempotent",
			function: testDeleteIdempotent,
			skipIf:   skipIfReadOnly,
		},
		{
			scenario: "listing secrets works",
			function: testList,
		},
		{
			scenario: "listing with name prefix filter",
			function: testListWithPrefix,
		},
		{
			scenario: "listing with max results",
			function: testListWithMaxResults,
		},
		{
			scenario: "creating secret with tags",
			function: testCreateWithTags,
			skipIf:   skipIfReadOnlyOrNoTags,
		},
		{
			scenario: "filtering secrets by tags",
			function: testListWithTagFilter,
			skipIf:   skipIfReadOnlyOrNoTags,
		},
		{
			scenario: "context cancellation is respected",
			function: testContextCancellation,
		},
	}

	for _, adapter := range adapters {
		t.Run(adapter.name, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.scenario, func(t *testing.T) {
					manager, err := loadManager(t)
					if err != nil {
						t.Fatal("unexpected error loading manager:", err)
					}

					if test.skipIf != nil {
						if skip, reason := test.skipIf(manager); skip {
							t.Skip(reason)
						}
					}

					manager = adapter.adapter.AdaptManager(manager)
					test.function(t, manager)
				})
			}
		})
	}
}

func skipIfReadOnly(m secret.Manager) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.CreateSecret(ctx, "test-readonly-check-"+randomString(), []byte("value"))
	if errors.Is(err, secret.ErrReadOnly) {
		return true, "backend is read-only"
	}
	// Clean up if successful
	if err == nil {
		_ = m.DeleteSecret(ctx, "test-readonly-check-"+randomString())
	}
	return false, ""
}

func skipIfReadOnlyOrNoTags(m secret.Manager) (bool, string) {
	// For read-only backends or backends without tag support,
	// we skip the tag tests
	if skip, reason := skipIfReadOnly(m); skip {
		return skip, reason
	}

	// Try creating a secret with tags to see if they're supported
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testName := "test-tag-support-" + randomString()
	_, err := m.CreateSecret(ctx, testName, []byte("test"), secret.Tag("test", "value"))
	if err == nil {
		// Clean up the test secret
		_ = m.DeleteSecret(ctx, testName)
		return false, "" // Tags are supported
	}

	// If we get here, assume no tag support (or other error)
	// Most backends support tags, so this is rare
	return false, ""
}

func testCreateAndGet(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name := "test-secret-" + randomString()
	value := []byte("secret-value-" + randomString())

	info, err := manager.CreateSecret(ctx, name, value)
	if err != nil {
		t.Fatal("unexpected error creating secret:", err)
	}

	if info.Name != name {
		t.Errorf("expected name %q, got %q", name, info.Name)
	}

	gotValue, gotInfo, err := manager.GetSecret(ctx, name)
	if err != nil {
		t.Fatal("unexpected error getting secret:", err)
	}

	if string(gotValue) != string(value) {
		t.Errorf("expected value %q, got %q", value, gotValue)
	}

	if gotInfo.Name != name {
		t.Errorf("expected name %q, got %q", name, gotInfo.Name)
	}

	// Clean up
	_ = manager.DeleteSecret(ctx, name)
}

func testGetNotExist(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	_, _, err := manager.GetSecret(ctx, "nonexistent-secret-"+randomString())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func testCreateDuplicate(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name := "test-duplicate-" + randomString()
	value := []byte("value")

	_, err := manager.CreateSecret(ctx, name, value)
	if err != nil {
		t.Fatal("unexpected error creating secret:", err)
	}
	defer manager.DeleteSecret(ctx, name)

	_, err = manager.CreateSecret(ctx, name, value)
	if err == nil {
		t.Fatal("expected error creating duplicate, got nil")
	}

	if !errors.Is(err, secret.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func testUpdate(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name := "test-update-" + randomString()
	initialValue := []byte("initial-value")
	newValue := []byte("new-value")

	_, err := manager.CreateSecret(ctx, name, initialValue)
	if err != nil {
		t.Fatal("unexpected error creating secret:", err)
	}
	defer manager.DeleteSecret(ctx, name)

	_, err = manager.UpdateSecret(ctx, name, newValue)
	if err != nil {
		t.Fatal("unexpected error updating secret:", err)
	}

	gotValue, _, err := manager.GetSecret(ctx, name)
	if err != nil {
		t.Fatal("unexpected error getting secret:", err)
	}

	if string(gotValue) != string(newValue) {
		t.Errorf("expected value %q, got %q", newValue, gotValue)
	}
}

func testDelete(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name := "test-delete-" + randomString()
	value := []byte("value")

	_, err := manager.CreateSecret(ctx, name, value)
	if err != nil {
		t.Fatal("unexpected error creating secret:", err)
	}

	err = manager.DeleteSecret(ctx, name)
	if err != nil {
		t.Fatal("unexpected error deleting secret:", err)
	}

	_, _, err = manager.GetSecret(ctx, name)
	if err == nil {
		t.Fatal("expected error getting deleted secret, got nil")
	}

	if !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func testDeleteIdempotent(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name := "test-delete-idempotent-" + randomString()

	// Delete non-existent secret should not error (idempotent)
	err := manager.DeleteSecret(ctx, name)
	// Some backends may return ErrNotFound, others may be truly idempotent
	// Both are acceptable
	if err != nil && !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("unexpected error deleting non-existent secret: %v", err)
	}
}

func testList(t *testing.T, manager secret.Manager) {
	ctx := context.Background()

	count := 0
	for _, err := range manager.ListSecrets(ctx) {
		if err != nil {
			t.Fatal("unexpected error listing secrets:", err)
		}
		count++
	}

	// Should successfully list secrets (count can be zero for empty lists)
	// Test passes as long as no error occurred
}

func testListWithPrefix(t *testing.T, manager secret.Manager) {
	ctx := context.Background()

	// For read-only backends, just test that prefix filtering works with existing vars
	var secrets []secret.Secret
	for s, err := range manager.ListSecrets(ctx, secret.NamePrefix("TEST_")) {
		if err != nil {
			t.Fatal("unexpected error listing secrets:", err)
		}
		secrets = append(secrets, s)
	}

	// All returned secrets should have the prefix
	for _, s := range secrets {
		if !hasPrefix(s.Name, "TEST_") {
			t.Errorf("secret %q does not have prefix TEST_", s.Name)
		}
	}
}

func testListWithMaxResults(t *testing.T, manager secret.Manager) {
	ctx := context.Background()

	var secrets []secret.Secret
	for s, err := range manager.ListSecrets(ctx, secret.MaxResults(5)) {
		if err != nil {
			t.Fatal("unexpected error listing secrets:", err)
		}
		secrets = append(secrets, s)
	}

	if len(secrets) > 5 {
		t.Errorf("expected at most 5 secrets, got %d", len(secrets))
	}
}

func testCreateWithTags(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name := "test-tags-" + randomString()
	value := []byte("value")
	tags := map[string]string{
		"env":     "test",
		"service": "api",
	}

	info, err := manager.CreateSecret(ctx, name, value, secret.Tags(tags))
	if err != nil {
		t.Fatal("unexpected error creating secret with tags:", err)
	}
	defer manager.DeleteSecret(ctx, name)

	if len(info.Tags) == 0 {
		t.Error("expected tags to be set, got empty tags")
	}

	// Verify tags are returned on Get
	_, gotInfo, err := manager.GetSecret(ctx, name)
	if err != nil {
		t.Fatal("unexpected error getting secret:", err)
	}

	if len(gotInfo.Tags) == 0 {
		t.Error("expected tags to be persisted, got empty tags")
	}
}

func testListWithTagFilter(t *testing.T, manager secret.Manager) {
	ctx := context.Background()
	name1 := "test-tag-filter-1-" + randomString()
	name2 := "test-tag-filter-2-" + randomString()

	// Create two secrets with different tags
	_, err := manager.CreateSecret(ctx, name1, []byte("value1"), secret.Tag("env", "prod"))
	if err != nil {
		t.Fatal("unexpected error creating secret 1:", err)
	}
	defer manager.DeleteSecret(ctx, name1)

	_, err = manager.CreateSecret(ctx, name2, []byte("value2"), secret.Tag("env", "dev"))
	if err != nil {
		t.Fatal("unexpected error creating secret 2:", err)
	}
	defer manager.DeleteSecret(ctx, name2)

	// List secrets with env=prod tag
	var secrets []secret.Secret
	for s, err := range manager.ListSecrets(ctx, secret.FilterByTag("env", "prod")) {
		if err != nil {
			t.Fatal("unexpected error listing secrets:", err)
		}
		secrets = append(secrets, s)
	}

	// Should find at least our secret
	found := false
	for _, s := range secrets {
		if s.Name == name1 {
			found = true
		}
		if s.Name == name2 {
			t.Errorf("found secret %q with wrong tag", name2)
		}
	}

	if !found {
		t.Error("did not find secret with matching tag")
	}
}

func testContextCancellation(t *testing.T, manager secret.Manager) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := manager.GetSecret(ctx, "any-secret")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// Helper functions

func randomString() string {
	return time.Now().Format("20060102-150405.000000")
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
