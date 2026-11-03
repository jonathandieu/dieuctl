package secrets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testKey = "sk-test-0123456789"

// serve answers by path from codes (default 404) with body, recording requests.
func serve(t *testing.T, codes map[string]int, body string) (*httptest.Server, *[]*http.Request) {
	t.Helper()
	var seen []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r)
		code, ok := codes[r.URL.Path]
		if !ok {
			code = 404
		}
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func check(c Checker) Outcome {
	return c.Check(context.Background(), Creds{"token": Secret(testKey)})
}

func assertOutcome(t *testing.T, got Outcome, status Status, detailPart string) {
	t.Helper()
	if got.Status != status || !strings.Contains(got.Detail, detailPart) {
		t.Fatalf("got %+v, want %s containing %q", got, status, detailPart)
	}
}

func TestDigitalOcean(t *testing.T) {
	allDenied := map[string]int{}
	for _, p := range []string{"/v2/account", "/v2/regions", "/v2/sizes", "/v2/projects", "/v2/droplets", "/v2/kubernetes/clusters", "/v2/domains"} {
		allDenied[p] = 403
	}
	cases := []struct {
		name   string
		codes  map[string]int
		status Status
		detail string
	}{
		{"full access", map[string]int{"/v2/account": 200}, StatusPass, "200"},
		{"scoped, account:read missing", map[string]int{"/v2/account": 403, "/v2/regions": 403, "/v2/sizes": 200}, StatusPass, "scoped"},
		{"revoked", map[string]int{"/v2/account": 401}, StatusFail, "401 invalid or revoked"},
		{"all forbidden", allDenied, StatusFail, "scoped token without read scopes"},
		{"vendor down", map[string]int{"/v2/account": 503}, StatusError, "503"},
		{"rate limited", map[string]int{"/v2/account": 429}, StatusError, "429"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := serve(t, tc.codes, "")
			assertOutcome(t, check(NewDigitalOcean(srv.Client(), srv.URL)), tc.status, tc.detail)
			if got := (*seen)[0].Header.Get("Authorization"); got != "Bearer "+testKey {
				t.Fatalf("auth header = %q", got)
			}
		})
	}
}

func TestOpenAIRestrictedKey(t *testing.T) {
	srv, _ := serve(t, map[string]int{"/v1/models": 403, "/v1/files": 200}, "")
	assertOutcome(t, check(NewOpenAI(srv.Client(), srv.URL)), StatusPass, "restricted key")

	srv, _ = serve(t, map[string]int{"/v1/models": 401}, "")
	assertOutcome(t, check(NewOpenAI(srv.Client(), srv.URL)), StatusFail, "401")
}

func TestAnthropic(t *testing.T) {
	srv, seen := serve(t, map[string]int{"/v1/models": 200}, "")
	assertOutcome(t, check(NewAnthropic(srv.Client(), srv.URL)), StatusPass, "200")
	r := (*seen)[0]
	if r.Header.Get("x-api-key") != testKey || r.Header.Get("anthropic-version") == "" {
		t.Fatalf("headers = %v", r.Header)
	}

	srv, _ = serve(t, map[string]int{"/v1/models": 401}, "")
	assertOutcome(t, check(NewAnthropic(srv.Client(), srv.URL)), StatusFail, "401")
}

func TestAnthropicAdminKeyUsesAdminAPI(t *testing.T) {
	srv, seen := serve(t, map[string]int{"/v1/organizations/me": 200}, "")
	out := NewAnthropic(srv.Client(), srv.URL).Check(context.Background(), Creds{"token": "sk-ant-admin01-abc"})
	assertOutcome(t, out, StatusPass, "200")
	if p := (*seen)[0].URL.Path; p != "/v1/organizations/me" {
		t.Fatalf("path = %s", p)
	}
}

func TestGitHub(t *testing.T) {
	srv, _ := serve(t, map[string]int{"/user": 200}, "")
	assertOutcome(t, check(NewGitHub(srv.Client(), srv.URL)), StatusPass, "200")
	srv, _ = serve(t, map[string]int{"/user": 401}, "")
	assertOutcome(t, check(NewGitHub(srv.Client(), srv.URL)), StatusFail, "401")
}

func TestTFCFallsBackToOrganizations(t *testing.T) {
	srv, _ := serve(t, map[string]int{"/api/v2/account/details": 404, "/api/v2/organizations": 200}, "")
	assertOutcome(t, check(NewTFC(srv.Client(), srv.URL)), StatusPass, "non-user token")

	srv, _ = serve(t, map[string]int{"/api/v2/account/details": 401, "/api/v2/organizations": 401}, "")
	assertOutcome(t, check(NewTFC(srv.Client(), srv.URL)), StatusFail, "401")
}

func TestCloudflare(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/client/v4/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":{"status":"active"}}`)
	})
	mux.HandleFunc("/client/v4/accounts/acc1/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":{"status":"expired"}}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	assertOutcome(t, check(NewCloudflare(ts.Client(), ts.URL, "")), StatusPass, "200")
	assertOutcome(t, check(NewCloudflare(ts.Client(), ts.URL, "acc1")), StatusFail, "expired")

	un, _ := serve(t, map[string]int{"/client/v4/user/tokens/verify": 401}, "")
	assertOutcome(t, check(NewCloudflare(un.Client(), un.URL, "")), StatusFail, "account_id")
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var hit bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer srv.Close()

	assertOutcome(t, check(NewGitHub(NewHTTPClient(), srv.URL)), StatusError, "302")
	if hit {
		t.Fatal("redirect target was contacted with the credential")
	}
}

func TestNetworkErrorIsInconclusive(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	assertOutcome(t, check(NewGitHub(NewHTTPClient(), url)), StatusError, "network error")
}
