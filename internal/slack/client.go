// Package slack is a minimal client for the Slack Web API methods dieuctl
// needs. It intentionally wraps plain net/http rather than pulling in a full
// SDK: only two read-only, officially documented methods are used here
// (emoji.list, auth.test).
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const baseURL = "https://slack.com/api"

// Client calls the Slack Web API with a single token (bot or user).
type Client struct {
	token      string
	httpClient *http.Client
}

// New creates a Client. token needs at least the emoji:read scope for EmojiList.
func New(token string) *Client {
	return &Client{token: token, httpClient: http.DefaultClient}
}

// Emoji maps custom emoji name to either an image URL, or "alias:<name>" if
// the emoji is an alias of another one.
type Emoji struct {
	Name  string
	Value string
}

// IsAlias reports whether this emoji is an alias of another emoji, and if so
// returns the target name.
func (e Emoji) IsAlias() (target string, ok bool) {
	const prefix = "alias:"
	if !strings.HasPrefix(e.Value, prefix) {
		return "", false
	}
	return strings.TrimPrefix(e.Value, prefix), true
}

// EmojiList returns every custom emoji visible to the token's workspace.
func (c *Client) EmojiList(ctx context.Context) (map[string]string, error) {
	var out struct {
		OK    bool              `json:"ok"`
		Error string            `json:"error"`
		Emoji map[string]string `json:"emoji"`
	}
	if err := c.get(ctx, "emoji.list", &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("slack emoji.list: %s", out.Error)
	}
	return out.Emoji, nil
}

// TeamName returns the current workspace's name, via auth.test. Callers
// should treat a failure here as non-fatal (it's used only for a manifest
// annotation), the token's own validity was already exercised by EmojiList.
func (c *Client) TeamName(ctx context.Context) (string, error) {
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Team  string `json:"team"`
	}
	if err := c.get(ctx, "auth.test", &out); err != nil {
		return "", err
	}
	if !out.OK {
		return "", fmt.Errorf("slack auth.test: %s", out.Error)
	}
	return out.Team, nil
}

func (c *Client) get(ctx context.Context, method string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/"+method, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack %s: %w", method, err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("slack %s: decode response: %w", method, err)
	}
	return nil
}
