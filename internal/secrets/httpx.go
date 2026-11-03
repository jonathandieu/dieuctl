package secrets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const maxBody = 64 << 10

// NewHTTPClient returns a client that never follows redirects, so a
// credential can't be replayed to a host the checker didn't pin.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func send(ctx context.Context, c *http.Client, req *http.Request) (int, []byte, error) {
	resp, err := c.Do(req.WithContext(ctx))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	return resp.StatusCode, body, nil
}

func netOutcome(err error) Outcome {
	var ne net.Error
	if (errors.As(err, &ne) && ne.Timeout()) || errors.Is(err, context.DeadlineExceeded) {
		return Outcome{StatusError, "timeout"}
	}
	return Outcome{StatusError, "network error"}
}

// classify maps an HTTP status to an Outcome using only fixed strings.
func classify(code int) Outcome {
	switch {
	case code >= 200 && code < 300:
		return Outcome{StatusPass, fmt.Sprintf("%d", code)}
	case code == 401:
		return Outcome{StatusFail, "401 invalid or revoked"}
	case code == 403:
		return Outcome{StatusFail, "403 forbidden"}
	case code == 429:
		return Outcome{StatusError, "429 rate limited"}
	case code >= 500:
		return Outcome{StatusError, fmt.Sprintf("%d vendor error", code)}
	default:
		return Outcome{StatusError, fmt.Sprintf("unexpected status %d", code)}
	}
}

// ladder probes read-only endpoints in order with one credential. A status in
// skip moves to the next rung (default: 403 only), so a scoped token that
// can't read the first endpoint still gets validated by a later one.
type ladder struct {
	client *http.Client
	base   string
	paths  []string
	skip   map[int]bool
	auth   func(r *http.Request, token string)

	scopedPass string // detail suffix when a rung after the first succeeds
	allDenied  string // FAIL detail when every rung answers 403
}

func (l ladder) check(ctx context.Context, token string) Outcome {
	skip := l.skip
	if skip == nil {
		skip = map[int]bool{403: true}
	}
	last := 0
	for i, p := range l.paths {
		req, err := http.NewRequest(http.MethodGet, l.base+p, nil)
		if err != nil {
			return Outcome{StatusError, "bad request"}
		}
		l.auth(req, token)
		code, _, err := send(ctx, l.client, req)
		if err != nil {
			return netOutcome(err)
		}
		last = code
		if code >= 200 && code < 300 {
			if i == 0 {
				return Outcome{StatusPass, fmt.Sprintf("%d", code)}
			}
			return Outcome{StatusPass, fmt.Sprintf("%d scoped, %s", code, l.scopedPass)}
		}
		if !skip[code] {
			return classify(code)
		}
	}
	if last == 403 && l.allDenied != "" {
		return Outcome{StatusFail, l.allDenied}
	}
	return classify(last)
}
