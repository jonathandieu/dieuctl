package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestParseShare(t *testing.T) {
	s, err := parseShare("111:42.00:21.00")
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != 111 || s.PaidShare != "42.00" || s.OwedShare != "21.00" {
		t.Fatalf("%+v", s)
	}
	if _, err := parseShare("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitwiseHelp(t *testing.T) {
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	rootCmd.SetArgs([]string{"splitwise", "--help"})
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !bytesContainsAll(out, "whoami", "groups", "expenses", "add") {
		t.Errorf("help output missing subcommands:\n%s", out)
	}
}

func bytesContainsAll(s string, needles ...string) bool {
	for _, n := range needles {
		if !bytes.Contains([]byte(s), []byte(n)) {
			return false
		}
	}
	return true
}

func TestSplitwiseWhoamiAndAddAgainstFakeAPI(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		cfgFile = ""
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/get_current_user":
			io.WriteString(w, `{"user":{"id":7,"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"}}`)
		case "/get_groups":
			io.WriteString(w, `{"groups":[{"id":3,"name":"Apartment","members":[{"id":7},{"id":8}]}]}`)
		case "/create_expense":
			io.WriteString(w, `{"expenses":[{"id":99,"cost":"42.00","description":"Groceries","group_id":3}],"errors":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("splitwise:\n  api_token: test-token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIEUCTL_SPLITWISE_BASE_URL", srv.URL)
	cfgFile = cfgPath

	run := func(args ...string) string {
		viper.Reset()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("%v\n%s", err, buf.String())
		}
		return buf.String()
	}

	who := run("splitwise", "whoami")
	if !bytesContainsAll(who, "Ada Lovelace", "ada@example.com", "7") {
		t.Errorf("whoami = %q", who)
	}

	add := run("splitwise", "add", "--cost", "42.00", "--description", "Groceries", "--group", "Apartment")
	if !bytesContainsAll(add, "created expense 99", "Groceries", "Apartment") {
		t.Errorf("add = %q", add)
	}
}
