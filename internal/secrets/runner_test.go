package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonathandieu/dieuctl/internal/op"
)

type fakeItems map[string]*op.Item

func (f fakeItems) Item(_ context.Context, _ string, title string) (*op.Item, error) {
	it, ok := f[title]
	if !ok {
		return nil, fmt.Errorf("%w: %s", op.ErrItemNotFound, title)
	}
	cp := *it
	return &cp, nil
}

type fakeChecker struct {
	verdict func(Creds) Outcome
	seen    *[]Creds
}

func (f fakeChecker) Check(_ context.Context, c Creds) Outcome {
	*f.seen = append(*f.seen, c)
	return f.verdict(c)
}

func fakeKinds(seen *[]Creds, verdict func(Creds) Outcome) map[string]Kind {
	mk := func(Params) Checker { return fakeChecker{verdict, seen} }
	return map[string]Kind{
		"token":  {New: mk},
		"spaces": {Fields: map[string]string{"access_key_id": "access_key_id", "secret_access_key": "secret_access_key"}, New: mk},
	}
}

func pass(Creds) Outcome { return Outcome{StatusPass, "200"} }

func TestMultipleKeysPerItemGetOneRowEach(t *testing.T) {
	items := fakeItems{"Anthropic": {Fields: []op.Field{
		{Label: "default/platform-production", Type: "CONCEALED", Value: "key-a"},
		{Label: "valid from", Type: "DATE", Value: "2026-01-01"},
		{Label: "intelligent-inference-shared/dev", Type: "CONCEALED", Value: "key-b"},
		{Label: "notesPlain", Type: "STRING", Value: "not a key"},
		{Label: "credential", Type: "CONCEALED", Value: ""},
	}}}
	var seen []Creds
	r := Runner{Items: items, Vault: "1Password", Kinds: fakeKinds(&seen, func(c Creds) Outcome {
		if c["token"] == "key-b" {
			return Outcome{StatusFail, "401 invalid or revoked"}
		}
		return Outcome{StatusPass, "200"}
	})}

	res := r.Run(context.Background(), []Target{{Name: "anthropic.api-key", Kind: "token", Items: []string{"anthropic.api-key", "Anthropic"}}})

	if len(res) != 2 || len(seen) != 2 {
		t.Fatalf("rows = %+v, probes = %d", res, len(seen))
	}
	if res[0].Item != "Anthropic" || res[0].Field != "default/platform-production" || res[0].Status != StatusPass {
		t.Fatalf("row0 = %+v", res[0])
	}
	if res[1].Field != "intelligent-inference-shared/dev" || res[1].Status != StatusFail {
		t.Fatalf("row1 = %+v", res[1])
	}
	if ExitCode(res) != 1 {
		t.Fatal("a dead key must exit 1")
	}
}

func TestNewTitleWinsOverLegacyAlias(t *testing.T) {
	items := fakeItems{
		"openai.api-key": {Fields: []op.Field{{Label: "credential", Type: "CONCEALED", Value: "new"}}},
		"OpenAI":         {Fields: []op.Field{{Label: "credential", Type: "CONCEALED", Value: "old"}}},
	}
	var seen []Creds
	r := Runner{Items: items, Kinds: fakeKinds(&seen, pass)}
	res := r.Run(context.Background(), []Target{{Name: "openai.api-key", Kind: "token", Items: []string{"openai.api-key", "OpenAI"}}})
	if res[0].Item != "openai.api-key" || seen[0]["token"] != "new" {
		t.Fatalf("res = %+v seen = %v", res, len(seen))
	}
}

func TestMissingItemAndEmptyFields(t *testing.T) {
	items := fakeItems{"OpenAI": {Fields: []op.Field{{Label: "credential", Type: "CONCEALED", Value: ""}}}}
	var seen []Creds
	r := Runner{Items: items, Kinds: fakeKinds(&seen, pass)}

	res := r.Run(context.Background(), []Target{
		{Name: "digitalocean.work", Kind: "token", Items: []string{"digitalocean.work"}},
		{Name: "openai.api-key", Kind: "token", Items: []string{"OpenAI"}},
		{Name: "pinned", Kind: "token", Items: []string{"OpenAI"}, Fields: []string{"credential", "other"}},
	})
	want := []struct {
		status Status
		detail string
	}{
		{StatusMissing, "item not found (tried digitalocean.work)"},
		{StatusMissing, "no non-empty key field"},
		{StatusMissing, "field is empty"},
		{StatusMissing, "field not found"},
	}
	if len(res) != len(want) {
		t.Fatalf("res = %+v", res)
	}
	for i, w := range want {
		if res[i].Status != w.status || !strings.Contains(res[i].Detail, w.detail) {
			t.Fatalf("row %d = %+v, want %v", i, res[i], w)
		}
	}
	if len(seen) != 0 {
		t.Fatal("empty or absent fields must not be probed")
	}
}

func TestSpacesPairResolved(t *testing.T) {
	items := fakeItems{"digitalocean.personal-spaces": {Fields: []op.Field{
		{Label: "credential", Type: "CONCEALED", Value: ""},
		{Label: "access_key_id", Type: "CONCEALED", Value: "id"},
		{Label: "secret_access_key", Type: "CONCEALED", Value: "sec"},
	}}}
	var seen []Creds
	r := Runner{Items: items, Kinds: fakeKinds(&seen, pass)}
	res := r.Run(context.Background(), []Target{{Name: "digitalocean.personal-spaces", Kind: "spaces", Items: []string{"digitalocean.personal-spaces"}}})
	if len(res) != 1 || res[0].Status != StatusPass || res[0].Field != "access_key_id,secret_access_key" {
		t.Fatalf("res = %+v", res)
	}
	if seen[0]["access_key_id"] != "id" || seen[0]["secret_access_key"] != "sec" {
		t.Fatal("creds not mapped by role")
	}

	items["digitalocean.personal-spaces"].Fields = items["digitalocean.personal-spaces"].Fields[:2]
	res = r.Run(context.Background(), []Target{{Name: "x", Kind: "spaces", Items: []string{"digitalocean.personal-spaces"}}})
	if res[0].Status != StatusMissing || !strings.Contains(res[0].Detail, "secret_access_key") {
		t.Fatalf("res = %+v", res)
	}
}

func TestLookupErrorIsInconclusiveNotMissing(t *testing.T) {
	r := Runner{Items: errItems{}, Kinds: fakeKinds(new([]Creds), pass)}
	res := r.Run(context.Background(), []Target{{Name: "a", Kind: "token", Items: []string{"a", "b"}}})
	if res[0].Status != StatusError {
		t.Fatalf("res = %+v", res)
	}
}

type errItems struct{}

func (errItems) Item(context.Context, string, string) (*op.Item, error) {
	return nil, errors.New("op item get: not signed in")
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		statuses []Status
		want     int
	}{
		{nil, 0},
		{[]Status{StatusPass}, 0},
		{[]Status{StatusPass, StatusError}, 2},
		{[]Status{StatusError, StatusMissing}, 1},
		{[]Status{StatusFail, StatusError}, 1},
	}
	for _, tc := range cases {
		var rs []Result
		for _, s := range tc.statuses {
			rs = append(rs, Result{Status: s})
		}
		if got := ExitCode(rs); got != tc.want {
			t.Fatalf("%v: got %d want %d", tc.statuses, got, tc.want)
		}
	}
}

// End to end with real checkers against a hostile server that echoes every
// header and the request line back in its body: no output form may contain
// any credential.
func TestNoSecretReachesOutput(t *testing.T) {
	const (
		tokenSecret = "tok-AAAA-very-secret"
		spaceID     = "DO00IDIDIDID"
		spaceSecret = "spaces-BBBB-very-secret"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b bytes.Buffer
		r.Header.Write(&b)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "<Error><Code>SignatureDoesNotMatch</Code><Message>%s %s</Message></Error>", r.URL.String(), b.String())
	}))
	defer srv.Close()

	kinds := map[string]Kind{
		"token": {New: func(Params) Checker {
			return bearerChecker{ladder{client: srv.Client(), base: srv.URL, paths: []string{"/a", "/b"}, auth: bearer, allDenied: "403 on all probes"}}
		}},
		"spaces": {
			Fields: map[string]string{"access_key_id": "access_key_id", "secret_access_key": "secret_access_key"},
			New: func(Params) Checker {
				return spacesChecker{client: srv.Client(), region: "nyc3", endpoint: srv.URL, now: time.Now}
			},
		},
	}
	items := fakeItems{
		"t": {Fields: []op.Field{{Label: "credential", Type: "CONCEALED", Value: tokenSecret}}},
		"s": {Fields: []op.Field{
			{Label: "access_key_id", Type: "CONCEALED", Value: spaceID},
			{Label: "secret_access_key", Type: "CONCEALED", Value: spaceSecret},
		}},
	}
	res := Runner{Items: items, Kinds: kinds}.Run(context.Background(), []Target{
		{Name: "t", Kind: "token", Items: []string{"t"}},
		{Name: "s", Kind: "spaces", Items: []string{"s"}},
	})

	var table, js bytes.Buffer
	WriteTable(&table, res)
	if err := WriteJSON(&js, res); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{table.String(), js.String(), fmt.Sprintf("%+v", res)} {
		for _, s := range []string{tokenSecret, spaceSecret, spaceID} {
			if strings.Contains(out, s) {
				t.Fatalf("output leaked %q:\n%s", s, out)
			}
		}
	}
	if ExitCode(res) != 1 {
		t.Fatal("403s must exit non-zero")
	}
}

type lockedItems struct{ calls int }

func (l *lockedItems) Item(context.Context, string, string) (*op.Item, error) {
	l.calls++
	return nil, fmt.Errorf("%w: authorization timeout", op.ErrUnauthorized)
}

func TestUnauthorizedStopsFurtherLookups(t *testing.T) {
	items := &lockedItems{}
	r := Runner{Items: items, Kinds: fakeKinds(new([]Creds), pass)}
	res := r.Run(context.Background(), []Target{
		{Name: "a", Kind: "token", Items: []string{"a"}},
		{Name: "b", Kind: "token", Items: []string{"b"}},
		{Name: "c", Kind: "token", Items: []string{"c"}},
	})
	if items.calls != 1 || len(res) != 3 {
		t.Fatalf("calls = %d rows = %+v", items.calls, res)
	}
	if res[1].Status != StatusError || !strings.Contains(res[1].Detail, "skipped") {
		t.Fatalf("row1 = %+v", res[1])
	}
	if ExitCode(res) != 2 {
		t.Fatal("locked vault is inconclusive, exit 2")
	}
}

func TestBuiltinFieldPinning(t *testing.T) {
	for _, tg := range BuiltinTargets() {
		switch tg.Kind {
		case "cloudflare", "github", "tfc":
			if len(tg.Fields) != 1 || tg.Fields[0] != "credential" {
				t.Fatalf("%s must be pinned to credential, got %v", tg.Name, tg.Fields)
			}
		case "anthropic", "openai", "digitalocean":
			if len(tg.Fields) != 0 {
				t.Fatalf("%s must probe every concealed field, got %v", tg.Name, tg.Fields)
			}
		}
	}
}

func TestPinnedFieldsIgnoreOtherConcealedFields(t *testing.T) {
	items := fakeItems{"cloudflare.api-token": {Fields: []op.Field{
		{Label: "credential", Type: "CONCEALED", Value: "tok"},
		{Label: "zone_id", Type: "CONCEALED", Value: "zone"},
	}}}
	var seen []Creds
	r := Runner{Items: items, Kinds: fakeKinds(&seen, pass)}
	res := r.Run(context.Background(), []Target{{Name: "cf", Kind: "token", Items: []string{"cloudflare.api-token"}, Fields: []string{"credential"}}})
	if len(res) != 1 || len(seen) != 1 || seen[0]["token"] != "tok" {
		t.Fatalf("res = %+v probes = %d", res, len(seen))
	}
}
