package op

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"
)

// Resolver resolves op:// references to plaintext values.
// Uses the 1Password SDK when OP_SERVICE_ACCOUNT_TOKEN is set (CI/headless),
// otherwise shells out to `op read` using the active CLI session (local).
type Resolver struct {
	client *onepassword.Client
}

// New creates a Resolver. If OP_SERVICE_ACCOUNT_TOKEN is set, the SDK client
// is initialised and used for all resolution. Otherwise, `op read` CLI is used.
func New(ctx context.Context) (*Resolver, error) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		// No service account — use CLI fallback. Validate op is on PATH.
		if _, err := exec.LookPath("op"); err != nil {
			return nil, fmt.Errorf("no OP_SERVICE_ACCOUNT_TOKEN set and `op` CLI not found — run `dieuctl auth login` or set OP_SERVICE_ACCOUNT_TOKEN")
		}
		return &Resolver{}, nil
	}

	client, err := onepassword.NewClient(
		ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("dieuctl", "v0.1.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("init 1Password SDK: %w", err)
	}
	return &Resolver{client: client}, nil
}

// Resolve returns the plaintext value for an op:// reference.
// Non-op:// strings are returned as-is.
func (r *Resolver) Resolve(ctx context.Context, ref string) (string, error) {
	if !strings.HasPrefix(ref, "op://") {
		return ref, nil
	}

	if r.client != nil {
		val, err := r.client.Secrets().Resolve(ctx, ref)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", ref, err)
		}
		return val, nil
	}

	// CLI fallback
	out, err := exec.CommandContext(ctx, "op", "read", ref).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("resolve %s: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("resolve %s: %w — run `dieuctl auth login`", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveMap resolves all op:// references in a string map, returning
// a new map with plaintext values. Non-op:// values are passed through unchanged.
func (r *Resolver) ResolveMap(ctx context.Context, m map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(m))
	for k, v := range m {
		resolved, err := r.Resolve(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", k, err)
		}
		out[k] = resolved
	}
	return out, nil
}
