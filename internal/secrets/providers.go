package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type bearerChecker struct{ l ladder }

func (b bearerChecker) Check(ctx context.Context, c Creds) Outcome {
	return b.l.check(ctx, c["token"].Reveal())
}

func bearer(r *http.Request, token string) { r.Header.Set("Authorization", "Bearer "+token) }

// Anthropic: GET /v1/models. Admin keys (sk-ant-admin...) are rejected by the
// Messages API surface, so they are probed against the Admin API instead.
type anthropicChecker struct {
	client *http.Client
	base   string
}

func NewAnthropic(client *http.Client, base string) Checker {
	return anthropicChecker{client, base}
}

func (a anthropicChecker) Check(ctx context.Context, c Creds) Outcome {
	key := c["token"].Reveal()
	auth := func(r *http.Request, k string) {
		r.Header.Set("x-api-key", k)
		r.Header.Set("anthropic-version", "2023-06-01")
	}
	paths := []string{"/v1/models?limit=1"}
	if strings.HasPrefix(key, "sk-ant-admin") {
		paths = []string{"/v1/organizations/me"}
	}
	return ladder{
		client: a.client, base: a.base, paths: paths, auth: auth,
		allDenied: "403 forbidden (key lacks permission)",
	}.check(ctx, key)
}

func NewOpenAI(client *http.Client, base string) Checker {
	return bearerChecker{ladder{
		client: client, base: base,
		paths: []string{"/v1/models", "/v1/files?limit=1", "/v1/fine_tuning/jobs?limit=1", "/v1/vector_stores?limit=1"},
		auth: func(r *http.Request, t string) {
			bearer(r, t)
			r.Header.Set("OpenAI-Beta", "assistants=v2")
		},
		scopedPass: "restricted key without Models permission",
		allDenied:  "403 on all probes (restricted key without read permissions)",
	}}
}

func NewDigitalOcean(client *http.Client, base string) Checker {
	return bearerChecker{ladder{
		client: client, base: base,
		paths: []string{
			"/v2/account", "/v2/regions", "/v2/sizes?per_page=1", "/v2/projects?per_page=1",
			"/v2/droplets?per_page=1", "/v2/kubernetes/clusters", "/v2/domains?per_page=1",
		},
		auth:       bearer,
		scopedPass: "no account:read scope",
		allDenied:  "403 on all probes (scoped token without read scopes)",
	}}
}

func NewGitHub(client *http.Client, base string) Checker {
	return bearerChecker{ladder{
		client: client, base: base, paths: []string{"/user"},
		auth: func(r *http.Request, t string) {
			bearer(r, t)
			r.Header.Set("X-GitHub-Api-Version", "2022-11-28")
			r.Header.Set("Accept", "application/vnd.github+json")
		},
	}}
}

// TFC: /account/details only answers for user tokens, so team and
// organization tokens fall through to the organizations listing.
func NewTFC(client *http.Client, base string) Checker {
	return bearerChecker{ladder{
		client: client, base: base,
		paths: []string{"/api/v2/account/details", "/api/v2/organizations"},
		skip:  map[int]bool{401: true, 403: true, 404: true},
		auth: func(r *http.Request, t string) {
			bearer(r, t)
			r.Header.Set("Content-Type", "application/vnd.api+json")
		},
		scopedPass: "non-user token",
	}}
}

// Cloudflare: token verify endpoint. Account-owned tokens need AccountID.
type cloudflareChecker struct {
	client    *http.Client
	base      string
	accountID string
}

func NewCloudflare(client *http.Client, base, accountID string) Checker {
	return cloudflareChecker{client, base, accountID}
}

func (cf cloudflareChecker) Check(ctx context.Context, c Creds) Outcome {
	path := "/client/v4/user/tokens/verify"
	if cf.accountID != "" {
		path = "/client/v4/accounts/" + url.PathEscape(cf.accountID) + "/tokens/verify"
	}
	req, err := http.NewRequest(http.MethodGet, cf.base+path, nil)
	if err != nil {
		return Outcome{StatusError, "bad request"}
	}
	bearer(req, c["token"].Reveal())
	code, body, err := send(ctx, cf.client, req)
	if err != nil {
		return netOutcome(err)
	}
	if code == 401 && cf.accountID == "" {
		return Outcome{StatusFail, "401 invalid, revoked, or account-owned (set account_id)"}
	}
	out := classify(code)
	if out.Status != StatusPass {
		return out
	}
	var v struct {
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &v) != nil {
		return Outcome{StatusError, "unexpected response"}
	}
	switch s := v.Result.Status; s {
	case "active":
		return out
	case "disabled", "expired":
		return Outcome{StatusFail, fmt.Sprintf("token %s", s)}
	default:
		return Outcome{StatusFail, "token not active"}
	}
}
