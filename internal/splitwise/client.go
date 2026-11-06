package splitwise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://secure.splitwise.com/api/v3.0"
	origin         = "https://secure.splitwise.com"
	userAgent      = "dieuctl (+https://github.com/jonathandieu/dieuctl)"
)

// authRefreshHint is returned on 401/302. It must never include cookie or token values.
const authRefreshHint = "splitwise returned %s; refresh 1Password item splitwise.session (cookie + csrf_token) from DevTools, or set splitwise.api_token if you have a personal API key"

// Auth is resolved Splitwise credentials. Bearer takes precedence over session cookies.
type Auth struct {
	Bearer string
	Cookie string
	CSRF   string
}

// NewAuth picks Bearer when apiToken is set, otherwise Cookie+CSRF session auth.
func NewAuth(apiToken, cookie, csrf string) (Auth, error) {
	apiToken = strings.TrimSpace(apiToken)
	cookie = strings.TrimSpace(cookie)
	csrf = strings.TrimSpace(csrf)
	if apiToken != "" {
		return Auth{Bearer: apiToken}, nil
	}
	if cookie != "" && csrf != "" {
		return Auth{Cookie: cookie, CSRF: csrf}, nil
	}
	if cookie != "" || csrf != "" {
		return Auth{}, fmt.Errorf("splitwise session auth needs both session (Cookie header) and csrf (X-CSRF-Token)")
	}
	return Auth{}, fmt.Errorf("configure splitwise.api_token, or splitwise.session and splitwise.csrf in ~/.config/dieuctl/config.yaml")
}

func (a Auth) bearer() bool { return a.Bearer != "" }

// Client talks to Splitwise /api/v3.0.
type Client struct {
	base   string
	auth   Auth
	http   *http.Client
	origin string
}

// New builds a client. httpClient may be nil. baseURL may be empty for DefaultBaseURL.
func New(baseURL string, auth Auth, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("DIEUCTL_SPLITWISE_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Client{base: strings.TrimRight(baseURL, "/"), auth: auth, http: httpClient, origin: origin}
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var out currentUserResponse
	if err := c.do(ctx, http.MethodGet, "/get_current_user", nil, nil, &out); err != nil {
		return User{}, err
	}
	if out.User.ID == 0 {
		return User{}, fmt.Errorf("splitwise returned no current user")
	}
	return out.User, nil
}

func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	var out groupsResponse
	if err := c.do(ctx, http.MethodGet, "/get_groups", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Groups, nil
}

func (c *Client) Expenses(ctx context.Context, groupID int64, limit int) ([]Expense, error) {
	q := url.Values{}
	if groupID != 0 {
		q.Set("group_id", strconv.FormatInt(groupID, 10))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out expensesResponse
	if err := c.do(ctx, http.MethodGet, "/get_expenses", q, nil, &out); err != nil {
		return nil, err
	}
	return out.Expenses, nil
}

func (c *Client) CreateExpense(ctx context.Context, in CreateExpenseInput) (Expense, error) {
	body, err := in.payload()
	if err != nil {
		return Expense{}, err
	}
	var out createExpenseResponse
	if err := c.do(ctx, http.MethodPost, "/create_expense", nil, body, &out); err != nil {
		return Expense{}, err
	}
	if err := apiErrors(out.Errors); err != nil {
		return Expense{}, err
	}
	if len(out.Expenses) == 0 {
		return Expense{}, fmt.Errorf("splitwise created no expense")
	}
	return out.Expenses[0], nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body map[string]any, dest any) error {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return fmt.Errorf("splitwise url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var rdr io.Reader
	contentType := ""
	if body != nil {
		if c.auth.bearer() {
			b, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("encode json: %w", err)
			}
			rdr = bytes.NewReader(b)
			contentType = "application/json"
		} else {
			rdr = strings.NewReader(formEncode(body).Encode())
			contentType = "application/x-www-form-urlencoded"
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), rdr)
	if err != nil {
		return fmt.Errorf("splitwise request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("splitwise request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read splitwise response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		return fmt.Errorf(authRefreshHint, resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("splitwise %s %s: HTTP %s", method, path, resp.Status)
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode splitwise response: %w", err)
	}
	return nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.auth.bearer() {
		req.Header.Set("Authorization", "Bearer "+c.auth.Bearer)
		return
	}
	req.Header.Set("Cookie", c.auth.Cookie)
	req.Header.Set("X-CSRF-Token", c.auth.CSRF)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.origin)
	req.Header.Set("Referer", c.origin+"/")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
}

func formEncode(body map[string]any) url.Values {
	v := url.Values{}
	for k, val := range body {
		switch t := val.(type) {
		case nil:
			continue
		case bool:
			v.Set(k, strconv.FormatBool(t))
		case int:
			v.Set(k, strconv.Itoa(t))
		case int64:
			v.Set(k, strconv.FormatInt(t, 10))
		case float64:
			v.Set(k, strconv.FormatFloat(t, 'f', -1, 64))
		default:
			v.Set(k, fmt.Sprint(t))
		}
	}
	return v
}
