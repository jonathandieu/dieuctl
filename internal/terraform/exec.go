package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Run shells out to `terraform <args>` in dir with the given extra env vars
// merged on top of the current environment. Stdout and stderr stream to the
// caller's terminal.
func Run(ctx context.Context, dir string, env map[string]string, args ...string) error {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnv(env)
	return cmd.Run()
}

// Outputs runs `terraform output -json` in dir and returns the decoded map.
func Outputs(ctx context.Context, dir string, env map[string]string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "terraform", "output", "-json")
	cmd.Dir = dir
	cmd.Env = mergeEnv(env)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("terraform output: %w", err)
	}

	// output -json returns a map of name → {value, type, sensitive}
	var raw map[string]struct {
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parse terraform output: %w", err)
	}

	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = fmt.Sprintf("%v", v.Value)
	}
	return out, nil
}

func mergeEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
