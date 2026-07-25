package do

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const apiBase = "https://api.digitalocean.com/v2"

type Client struct {
	token      string
	httpClient *http.Client
}

func New(token string) *Client {
	return &Client{token: token, httpClient: &http.Client{}}
}

// FetchKubeconfig fetches a fresh kubeconfig for the given cluster ID from the DO API.
func (c *Client) FetchKubeconfig(ctx context.Context, clusterID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/kubernetes/clusters/%s/kubeconfig", apiBase, clusterID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DO API %d: %s", resp.StatusCode, body)
	}
	return io.ReadAll(resp.Body)
}

// SaveKubeconfig writes the kubeconfig to ~/.kube/<clusterName>.yaml, merges it
// into ~/.kube/config, and sets current-context to the new cluster.
func SaveKubeconfig(clusterName string, data []byte) (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".kube")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, clusterName+".yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	if err := mergeKubeconfig(filepath.Join(dir, "config"), data); err != nil {
		return path, fmt.Errorf("merge into ~/.kube/config: %w", err)
	}
	return path, nil
}

// mergeKubeconfig merges newData into the kubeconfig at dst, replacing any
// existing entries with the same name and setting current-context.
func mergeKubeconfig(dst string, newData []byte) error {
	var incoming map[string]interface{}
	if err := yaml.Unmarshal(newData, &incoming); err != nil {
		return fmt.Errorf("parse new kubeconfig: %w", err)
	}

	existing := map[string]interface{}{
		"apiVersion":     "v1",
		"kind":           "Config",
		"clusters":       []interface{}{},
		"users":          []interface{}{},
		"contexts":       []interface{}{},
		"current-context": "",
		"preferences":    map[string]interface{}{},
	}
	if raw, err := os.ReadFile(dst); err == nil {
		if err := yaml.Unmarshal(raw, &existing); err != nil {
			return fmt.Errorf("parse existing kubeconfig: %w", err)
		}
	}

	for _, key := range []string{"clusters", "users", "contexts"} {
		existing[key] = mergeNamedList(toList(existing[key]), toList(incoming[key]))
	}
	if ctx, ok := incoming["current-context"].(string); ok && ctx != "" {
		existing["current-context"] = ctx
	}

	out, err := yaml.Marshal(existing)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, out, 0600)
}

// mergeNamedList merges src entries into dst, replacing by name.
func mergeNamedList(dst, src []interface{}) []interface{} {
	index := map[string]int{}
	for i, item := range dst {
		if m, ok := item.(map[string]interface{}); ok {
			if name, ok := m["name"].(string); ok {
				index[name] = i
			}
		}
	}
	for _, item := range src {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := m["name"].(string)
		if !ok {
			continue
		}
		if i, exists := index[name]; exists {
			dst[i] = item
		} else {
			index[name] = len(dst)
			dst = append(dst, item)
		}
	}
	return dst
}

func toList(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	l, _ := v.([]interface{})
	return l
}
