package op

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrItemNotFound is returned by ItemReader when no item matches the title.
var ErrItemNotFound = errors.New("item not found")

// ErrUnauthorized is returned when the op CLI can't get a session (locked
// vault, dismissed or timed-out prompt), so further lookups would fail too.
var ErrUnauthorized = errors.New("1Password not authorized")

// Field is one field of a 1Password item. Value is only meaningful for string
// fields; callers must never log it.
type Field struct {
	Label string
	Type  string
	Value string
}

// Item is the subset of a 1Password item needed to enumerate credentials.
type Item struct {
	Title  string
	Fields []Field
}

// ItemReader fetches whole items by title. Unlike Resolver.Resolve it can
// enumerate fields, so labels containing "/" (which op:// would parse as a
// section separator) stay addressable.
type ItemReader struct{}

// Item fetches an item by title from the vault via the op CLI, which also
// honours OP_SERVICE_ACCOUNT_TOKEN.
func (ItemReader) Item(ctx context.Context, vault, title string) (*Item, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "op", "item", "get", title, "--vault", vault, "--format", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := firstLine(stderr.String())
		if strings.Contains(msg, "isn't an item") || strings.Contains(msg, "isn't a vault") {
			return nil, fmt.Errorf("%w: %s", ErrItemNotFound, title)
		}
		if msg == "" {
			msg = err.Error()
		}
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "authorization") || strings.Contains(lower, "signed in") || strings.Contains(lower, "sign in") {
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, msg)
		}
		return nil, fmt.Errorf("op item get: %s", msg)
	}
	return ParseItem(stdout.Bytes())
}

// ParseItem decodes `op item get --format json` output.
func ParseItem(data []byte) (*Item, error) {
	var raw struct {
		Title  string `json:"title"`
		Fields []struct {
			Label string          `json:"label"`
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.New("op item get: unexpected output")
	}
	item := &Item{Title: raw.Title}
	for _, f := range raw.Fields {
		var v string
		if len(f.Value) > 0 {
			// Non-string values (dates etc.) are left empty.
			_ = json.Unmarshal(f.Value, &v)
		}
		item.Fields = append(item.Fields, Field{Label: f.Label, Type: f.Type, Value: v})
	}
	return item, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
