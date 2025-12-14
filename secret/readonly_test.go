package secret

import (
	"context"
	"errors"
	"testing"
)

func TestReadOnly(t *testing.T) {
	ctx := context.Background()
	base := &mockManagerWithList{secrets: map[string]Value{
		"existing": Value("value"),
	}}

	ro := ReadOnly(base)

	t.Run("Get allows read", func(t *testing.T) {
		value, info, err := ro.GetSecret(ctx, "existing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(value) != "value" {
			t.Errorf("expected value 'value', got %q", value)
		}
		if info.Name != "existing" {
			t.Errorf("expected name 'existing', got %q", info.Name)
		}
	})

	t.Run("Create returns ErrReadOnly", func(t *testing.T) {
		_, err := ro.CreateSecret(ctx, "new", Value("value"))
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Update returns ErrReadOnly", func(t *testing.T) {
		_, err := ro.UpdateSecret(ctx, "existing", Value("new-value"))
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Delete returns ErrReadOnly", func(t *testing.T) {
		err := ro.DeleteSecret(ctx, "existing")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("DestroyVersion returns ErrReadOnly", func(t *testing.T) {
		err := ro.DestroySecretVersion(ctx, "existing", "v1")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("List allows read", func(t *testing.T) {
		count := 0
		for _, err := range ro.ListSecrets(ctx) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			count++
		}
		if count == 0 {
			t.Error("expected at least one secret")
		}
	})
}

func TestWithReadOnly(t *testing.T) {
	base := &mockManager{secrets: make(map[string]Value)}
	adapter := WithReadOnly()

	ro := adapter.AdaptManager(base)

	if ro == base {
		t.Error("expected adapter to return a different manager")
	}

	ctx := context.Background()
	_, err := ro.CreateSecret(ctx, "test", Value("value"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReadOnly) {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}
}
