package argocd

import "testing"

func TestParseClusterLabels(t *testing.T) {
	data := []byte(`
name: dieubernetes-prod-do-atl1
labels:
  purpose: prod
`)
	labels, err := parseClusterLabels(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if labels["purpose"] != "prod" {
		t.Errorf("labels[\"purpose\"] = %q, want %q", labels["purpose"], "prod")
	}
}

func TestParseClusterLabels_NoLabels(t *testing.T) {
	labels, err := parseClusterLabels([]byte(`name: dieubernetes-prod-do-atl1`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 0 {
		t.Errorf("labels = %v, want empty", labels)
	}
}

func TestParseClusterLabels_Invalid(t *testing.T) {
	if _, err := parseClusterLabels([]byte("not: valid: yaml: at: all:")); err == nil {
		t.Error("expected an error for invalid YAML, got nil")
	}
}

func TestParseCurrentContext(t *testing.T) {
	data := []byte(`
apiVersion: v1
kind: Config
current-context: do-atl1-dieubernetes-prod-do-atl1
`)
	ctx, err := parseCurrentContext(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "do-atl1-dieubernetes-prod-do-atl1"
	if ctx != want {
		t.Errorf("context = %q, want %q", ctx, want)
	}
}

func TestParseCurrentContext_Missing(t *testing.T) {
	_, err := parseCurrentContext([]byte(`apiVersion: v1
kind: Config
`))
	if err == nil {
		t.Error("expected an error when current-context is unset, got nil")
	}
}
