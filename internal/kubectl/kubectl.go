package kubectl

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

// Client runs kubectl with a fixed kubeconfig. kubectl must be in PATH —
// dieuctl lists it as a prerequisite alongside terraform and op.
type Client struct {
	kubeconfigPath string
}

// New checks that kubectl is available in PATH before returning a Client, so
// callers get a clear error at construction time rather than mid-operation.
func New(kubeconfigPath string) (*Client, error) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, fmt.Errorf("kubectl not found in PATH — install it from https://kubernetes.io/docs/tasks/tools/")
	}
	return &Client{kubeconfigPath: kubeconfigPath}, nil
}

// Apply applies a YAML manifest via kubectl apply -f -.
func (c *Client) Apply(ctx context.Context, manifest string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", c.kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

// EnsureNamespace creates a namespace idempotently.
func (c *Client) EnsureNamespace(ctx context.Context, name string) error {
	return c.Apply(ctx, fmt.Sprintf("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n", name))
}

// CreateOpaqueSecret creates or updates an Opaque secret. Values are base64-encoded
// into the data field so arbitrary bytes (including JSON) are safe.
func (c *Client) CreateOpaqueSecret(ctx context.Context, ns, name string, data map[string][]byte) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s\n  namespace: %s\ntype: Opaque\ndata:\n", name, ns)
	for k, v := range data {
		fmt.Fprintf(&sb, "  %s: %s\n", k, base64.StdEncoding.EncodeToString(v))
	}
	return c.Apply(ctx, sb.String())
}
