package cluster

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathandieu/dieuctl/internal/gitops"
)

func TestGenerateDieubernetesFiles(t *testing.T) {
	dir := t.TempDir()
	vars := gitops.ClusterVars{
		Name:      "dieubernetes-platform-do-nyc3",
		ShortName: "platform-do-nyc3",
		Provider:  "do",
		Account:   "personal",
		Region:    "nyc3",
		Tier:      "platform",
		LBID:      "abc-123",
		LBIP:      "1.2.3.4",
	}

	if err := generateDieubernetesFiles(dir, vars); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterDir := filepath.Join(dir, "clusters", vars.ShortName)
	files := []string{
		"cluster.yaml",
		filepath.Join("overrides", "envoy-gateway.yaml"),
		filepath.Join("overrides", "metrics-server.yaml"),
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(clusterDir, f)); err != nil {
			t.Errorf("expected file %q to exist: %v", f, err)
		}
	}

	// metrics-server override must contain the DO-specific flag
	got, err := os.ReadFile(filepath.Join(clusterDir, "overrides", "metrics-server.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const wantFlag = "--kubelet-insecure-tls"
	if !strings.Contains(string(got), wantFlag) {
		t.Errorf("metrics-server.yaml missing %q:\n%s", wantFlag, got)
	}

	// cluster.yaml's name field must be the short form (no dieubernetes- prefix) —
	// this is what ArgoCD registers and what dieuctl argocd register looks up.
	clusterYAML, err := os.ReadFile(filepath.Join(clusterDir, "cluster.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(clusterYAML), "name: platform-do-nyc3") {
		t.Errorf("cluster.yaml name field should be short form %q:\n%s", vars.ShortName, clusterYAML)
	}
}

func TestUpdatePrimaryClusterWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		workspace string
		want      string
	}{
		{
			name: "updates existing value",
			initial: `cloudflare_account_id     = "abc123"
primary_cluster_workspace = "dieubernetes-platform-do-atl1"
`,
			workspace: "dieubernetes-platform-do-nyc3",
			want: `cloudflare_account_id     = "abc123"
primary_cluster_workspace = "dieubernetes-platform-do-nyc3"
`,
		},
		{
			name: "appends when key missing",
			initial: `cloudflare_account_id = "abc123"
`,
			workspace: "dieubernetes-platform-do-atl1",
			want: `cloudflare_account_id = "abc123"

primary_cluster_workspace = "dieubernetes-platform-do-atl1"
`,
		},
		{
			name:      "handles empty file",
			initial:   "",
			workspace: "dieubernetes-platform-do-atl1",
			want:      "\nprimary_cluster_workspace = \"dieubernetes-platform-do-atl1\"\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tvarsPath := filepath.Join(dir, "terraform.tfvars")
			if err := os.WriteFile(tvarsPath, []byte(tc.initial), 0644); err != nil {
				t.Fatal(err)
			}
			if err := updatePrimaryClusterWorkspace(dir, tc.workspace); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, err := os.ReadFile(tvarsPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}
