package splitwise

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAuth_BearerWins(t *testing.T) {
	a, err := NewAuth("token", "cookie=1", "csrf")
	if err != nil {
		t.Fatal(err)
	}
	if !a.bearer() || a.Bearer != "token" || a.Cookie != "" {
		t.Fatalf("got %+v, want bearer-only", a)
	}
}

func TestNewAuth_CookieRequiresCSRF(t *testing.T) {
	if _, err := NewAuth("", "cookie=1", ""); err == nil {
		t.Fatal("expected error when csrf is missing")
	}
	a, err := NewAuth("", "cookie=1", "csrf")
	if err != nil {
		t.Fatal(err)
	}
	if a.bearer() || a.Cookie != "cookie=1" || a.CSRF != "csrf" {
		t.Fatalf("got %+v", a)
	}
}

func TestNewAuth_Empty(t *testing.T) {
	if _, err := NewAuth("", "", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_BaseURLFromEnv(t *testing.T) {
	t.Setenv("DIEUCTL_SPLITWISE_BASE_URL", "http://example.test/api")
	c := New("", Auth{Bearer: "k"}, nil)
	if c.base != "http://example.test/api" {
		t.Fatalf("base = %q", c.base)
	}
}

func TestClient_BearerHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		if r.Header.Get("Cookie") != "" {
			t.Errorf("bearer request must not send Cookie")
		}
		io.WriteString(w, `{"user":{"id":7,"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, Auth{Bearer: "secret-token"}, srv.Client())
	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 7 || u.DisplayName() != "Ada Lovelace" {
		t.Fatalf("user = %+v", u)
	}
	if got.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Authorization = %q", got.Get("Authorization"))
	}
	if strings.Contains(got.Get("Authorization"), "cookie") {
		t.Error("authorization header leaked a cookie")
	}
}

func TestClient_CookieHeadersOnWrite(t *testing.T) {
	var (
		gotHeader http.Header
		gotBody   string
		gotCT     string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, `{"expenses":[{"id":9,"cost":"25.00","description":"Groceries","group_id":3}],"errors":{}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, Auth{Cookie: "secret-cookie", CSRF: "secret-csrf"}, srv.Client())
	exp, err := c.CreateExpense(context.Background(), CreateExpenseInput{
		Cost:         "25.00",
		Description:  "Groceries",
		GroupID:      3,
		SplitEqually: true,
		CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.ID != 9 {
		t.Fatalf("expense = %+v", exp)
	}
	if gotHeader.Get("Cookie") != "secret-cookie" {
		t.Errorf("Cookie = %q", gotHeader.Get("Cookie"))
	}
	if gotHeader.Get("X-CSRF-Token") != "secret-csrf" {
		t.Errorf("X-CSRF-Token = %q", gotHeader.Get("X-CSRF-Token"))
	}
	if gotHeader.Get("Authorization") != "" {
		t.Errorf("cookie mode must not send Authorization, got %q", gotHeader.Get("Authorization"))
	}
	if gotHeader.Get("X-Requested-With") != "XMLHttpRequest" {
		t.Errorf("X-Requested-With = %q", gotHeader.Get("X-Requested-With"))
	}
	if !strings.Contains(gotCT, "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if !strings.Contains(gotBody, "split_equally=true") || !strings.Contains(gotBody, "group_id=3") {
		t.Errorf("form body = %q", gotBody)
	}
}

func TestClient_BearerCreateJSON(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, `{"expenses":[{"id":1,"cost":"10.00","description":"x","group_id":2}],"errors":{}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, Auth{Bearer: "k"}, srv.Client())
	_, err := c.CreateExpense(context.Background(), CreateExpenseInput{
		Cost:         "10.00",
		Description:  "x",
		GroupID:      2,
		SplitEqually: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if raw["split_equally"] != true || raw["cost"] != "10.00" {
		t.Fatalf("payload = %#v", raw)
	}
}

func TestClient_CreateExpenseShares(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&raw)
		io.WriteString(w, `{"expenses":[{"id":2,"cost":"10.00","description":"x","group_id":2}],"errors":{}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, Auth{Bearer: "k"}, srv.Client())
	_, err := c.CreateExpense(context.Background(), CreateExpenseInput{
		Cost:        "10.00",
		Description: "x",
		GroupID:     2,
		Shares: []Share{
			{UserID: 1, PaidShare: "10.00", OwedShare: "5.00"},
			{UserID: 2, PaidShare: "0", OwedShare: "5.00"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if raw["users__0__user_id"] != float64(1) || raw["users__1__owed_share"] != "5.00" {
		t.Fatalf("payload = %#v", raw)
	}
	if _, ok := raw["split_equally"]; ok {
		t.Fatal("split_equally should be omitted when shares are set")
	}
}

func TestClient_UnauthorizedDoesNotLeakSecrets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"not logged in"}`)
	}))
	t.Cleanup(srv.Close)

	secret := "super-secret-cookie-value"
	c := New(srv.URL, Auth{Cookie: secret, CSRF: "csrf-secret"}, srv.Client())
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, secret) || strings.Contains(msg, "csrf-secret") {
		t.Errorf("error leaked a secret: %s", msg)
	}
	if !strings.Contains(msg, "splitwise.session") {
		t.Errorf("error = %q, want refresh hint", msg)
	}
}

func TestClient_RedirectIsAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://secure.splitwise.com/login", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	c := New(srv.URL, Auth{Bearer: "k"}, client)
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "302") && !strings.Contains(err.Error(), "Found") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestClient_CreateExpenseAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"expenses":[],"errors":{"base":["cost is invalid"]}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, Auth{Bearer: "k"}, srv.Client())
	_, err := c.CreateExpense(context.Background(), CreateExpenseInput{
		Cost:         "x",
		Description:  "y",
		GroupID:      1,
		SplitEqually: true,
	})
	if err == nil || !strings.Contains(err.Error(), "cost is invalid") {
		t.Fatalf("error = %v", err)
	}
}

func TestClient_ExpensesQuery(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		io.WriteString(w, `{"expenses":[]}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, Auth{Bearer: "k"}, srv.Client())
	_, err := c.Expenses(context.Background(), 44, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "group_id=44") || !strings.Contains(gotURL, "limit=5") {
		t.Errorf("url = %q", gotURL)
	}
}

func TestFindGroup(t *testing.T) {
	groups := []Group{{ID: 1, Name: "Apartment"}, {ID: 2, Name: "Trip"}}
	g, err := FindGroup(groups, "apartment")
	if err != nil || g.ID != 1 {
		t.Fatalf("name match: %v %+v", err, g)
	}
	g, err = FindGroup(groups, "2")
	if err != nil || g.Name != "Trip" {
		t.Fatalf("id match: %v %+v", err, g)
	}
	if _, err := FindGroup(groups, "missing"); err == nil {
		t.Fatal("expected missing error")
	}
}

func TestCreateExpenseInputValidate(t *testing.T) {
	err := (CreateExpenseInput{Cost: "1", Description: "x"}).validate()
	if err == nil {
		t.Fatal("expected error without split or shares")
	}
}
