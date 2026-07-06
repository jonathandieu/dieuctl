package op

import (
	"context"
	"testing"
)

func TestResolve_PassThrough(t *testing.T) {
	r := &Resolver{}
	got, err := r.Resolve(context.Background(), "not-an-op-ref")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "not-an-op-ref" {
		t.Errorf("Resolve() = %q, want %q", got, "not-an-op-ref")
	}
}

func TestResolveMap_PassThrough(t *testing.T) {
	r := &Resolver{}
	in := map[string]string{
		"a": "plain-value",
		"b": "another-plain-value",
	}
	got, err := r.ResolveMap(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["a"] != "plain-value" || got["b"] != "another-plain-value" {
		t.Errorf("ResolveMap() = %v, want %v", got, in)
	}
}
