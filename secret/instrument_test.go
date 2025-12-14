package secret

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInstrument(t *testing.T) {
	// Setup a test tracer
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(trace.NewTracerProvider())

	ctx := context.Background()
	base := &mockManagerWithList{secrets: map[string]Value{
		"existing": Value("value"),
	}}

	instrumented := Instrument(base)

	t.Run("Create creates span", func(t *testing.T) {
		exporter.Reset()

		_, err := instrumented.CreateSecret(ctx, "new-secret", Value("new-value"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}

		span := spans[0]
		if span.Name != "secret.Create" {
			t.Errorf("expected span name 'secret.Create', got %q", span.Name)
		}

		// Check attributes
		attrs := span.Attributes
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == "secret.name" && attr.Value.AsString() == "new-secret" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected secret.name attribute")
		}
	})

	t.Run("Get creates span", func(t *testing.T) {
		exporter.Reset()

		_, _, err := instrumented.GetSecret(ctx, "existing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}

		if spans[0].Name != "secret.Get" {
			t.Errorf("expected span name 'secret.Get', got %q", spans[0].Name)
		}
	})

	t.Run("List creates span", func(t *testing.T) {
		exporter.Reset()

		count := 0
		for _, err := range instrumented.ListSecrets(ctx) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			count++
		}

		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}

		if spans[0].Name != "secret.List" {
			t.Errorf("expected span name 'secret.List', got %q", spans[0].Name)
		}
	})

	t.Run("error records error", func(t *testing.T) {
		exporter.Reset()

		// Try to get non-existent secret
		_, _, err := instrumented.GetSecret(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error")
		}

		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}

		span := spans[0]
		if span.Status.Code.String() != "Error" {
			t.Errorf("expected error status, got %q", span.Status.Code)
		}
	})
}

func TestWithInstrumentation(t *testing.T) {
	base := &mockManager{secrets: make(map[string]Value)}
	adapter := WithInstrumentation()

	instrumented := adapter.AdaptManager(base)

	if instrumented == base {
		t.Error("expected adapter to return a different manager")
	}

	// Verify it's instrumented
	_, ok := instrumented.(*instrumentedManager)
	if !ok {
		t.Error("expected instrumentedManager type")
	}
}
