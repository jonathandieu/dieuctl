package secrets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func spacesCreds() Creds {
	return Creds{"access_key_id": "DO00TESTACCESSID", "secret_access_key": "test-secret-key-value"}
}

func spacesServer(t *testing.T, code int, s3Code string) (*httptest.Server, *http.Request) {
	t.Helper()
	var got http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r.Clone(context.Background())
		w.WriteHeader(code)
		if s3Code != "" {
			fmt.Fprintf(w, `<?xml version="1.0"?><Error><Code>%s</Code><Message>secret detail</Message></Error>`, s3Code)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func newTestSpaces(srv *httptest.Server, bucket string) Checker {
	return spacesChecker{
		client: srv.Client(), region: "nyc3", bucket: bucket, endpoint: srv.URL,
		now: func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) },
	}
}

func TestSpaces(t *testing.T) {
	cases := []struct {
		name   string
		code   int
		s3Code string
		bucket string
		status Status
		detail string
	}{
		{"valid", 200, "", "", StatusPass, "200"},
		{"unknown key id", 403, "InvalidAccessKeyId", "", StatusFail, "invalid access key id"},
		{"wrong secret", 403, "SignatureDoesNotMatch", "", StatusFail, "wrong secret key"},
		{"scoped key", 403, "AccessDenied", "", StatusFail, "set bucket"},
		{"bucket head ok", 200, "", "loki", StatusPass, "200"},
		{"bucket missing", 404, "", "loki", StatusFail, "bucket not found"},
		{"vendor down", 503, "", "", StatusError, "503"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, req := spacesServer(t, tc.code, tc.s3Code)
			out := newTestSpaces(srv, tc.bucket).Check(context.Background(), spacesCreds())
			assertOutcome(t, out, tc.status, tc.detail)
			if strings.Contains(out.Detail, "secret detail") {
				t.Fatal("response body leaked into detail")
			}
			auth := req.Header.Get("Authorization")
			if !strings.Contains(auth, "Credential=DO00TESTACCESSID/20261102/nyc3/s3/aws4_request") {
				t.Fatalf("Authorization = %s", auth)
			}
			if strings.Contains(auth, "test-secret-key-value") {
				t.Fatal("secret key in Authorization header")
			}
			wantMethod := http.MethodGet
			if tc.bucket != "" {
				wantMethod = http.MethodHead
			}
			if req.Method != wantMethod {
				t.Fatalf("method = %s", req.Method)
			}
		})
	}
}

func TestSpacesDefaultRegion(t *testing.T) {
	c := NewSpaces(http.DefaultClient, "", "").(spacesChecker)
	if c.region != "nyc3" {
		t.Fatalf("region = %s", c.region)
	}
}
