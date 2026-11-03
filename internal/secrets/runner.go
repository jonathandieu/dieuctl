package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jonathandieu/dieuctl/internal/op"
)

// ItemSource fetches 1Password items by title.
type ItemSource interface {
	Item(ctx context.Context, vault, title string) (*op.Item, error)
}

const concealed = "CONCEALED"

type Runner struct {
	Items ItemSource
	Vault string
	Kinds map[string]Kind
}

// Run checks every target in order. It never returns an error: problems are
// reported per row.
func (r Runner) Run(ctx context.Context, targets []Target) []Result {
	var results []Result
	for i, t := range targets {
		rows := r.runTarget(ctx, t)
		results = append(results, rows...)
		if len(rows) == 1 && rows[0].Status == StatusError && strings.HasPrefix(rows[0].Detail, op.ErrUnauthorized.Error()) {
			// No 1Password session: every remaining lookup would stall on the same prompt.
			for _, rest := range targets[i+1:] {
				results = append(results, Result{Name: rest.Name, Status: StatusError, Detail: "skipped, 1Password not authorized"})
			}
			break
		}
	}
	return results
}

func (r Runner) runTarget(ctx context.Context, t Target) []Result {
	kind, ok := r.Kinds[t.Kind]
	if !ok {
		return []Result{{Name: t.Name, Status: StatusError, Detail: "unknown checker " + t.Kind}}
	}

	item, err := r.fetch(ctx, t)
	if err != nil {
		status := StatusError
		if errors.Is(err, op.ErrItemNotFound) {
			status = StatusMissing
		}
		return []Result{{Name: t.Name, Status: status, Detail: err.Error()}}
	}

	checker := kind.New(t.Params)
	if kind.Fields != nil {
		return []Result{r.checkRoles(ctx, t, item, kind, checker)}
	}
	return r.checkTokens(ctx, t, item, checker)
}

// fetch tries each candidate title in order. When none exists the returned
// error wraps op.ErrItemNotFound.
func (r Runner) fetch(ctx context.Context, t Target) (*op.Item, error) {
	for _, title := range t.Items {
		item, err := r.Items.Item(ctx, r.Vault, title)
		if err == nil {
			item.Title = title
			return item, nil
		}
		if !errors.Is(err, op.ErrItemNotFound) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w (tried %s)", op.ErrItemNotFound, strings.Join(t.Items, ", "))
}

func (r Runner) checkTokens(ctx context.Context, t Target, item *op.Item, c Checker) []Result {
	pinned := len(t.Fields) > 0
	wanted := make(map[string]bool, len(t.Fields))
	for _, f := range t.Fields {
		wanted[f] = true
	}
	seen := make(map[string]bool, len(t.Fields))

	var results []Result
	for _, f := range item.Fields {
		if pinned {
			if !wanted[f.Label] {
				continue
			}
			seen[f.Label] = true
		} else if f.Type != concealed {
			continue
		}
		if f.Value == "" {
			if pinned {
				results = append(results, missing(t, item, f.Label, "field is empty"))
			}
			continue
		}
		label := f.Label
		if label == "" {
			label = "(unlabeled)"
		}
		out := c.Check(ctx, Creds{"token": Secret(f.Value)})
		results = append(results, Result{Name: t.Name, Item: item.Title, Field: label, Status: out.Status, Detail: out.Detail})
	}
	for _, label := range t.Fields {
		if !seen[label] {
			results = append(results, missing(t, item, label, "field not found"))
		}
	}
	if len(results) == 0 {
		results = append(results, missing(t, item, "", "no non-empty key field"))
	}
	return results
}

func (r Runner) checkRoles(ctx context.Context, t Target, item *op.Item, kind Kind, c Checker) Result {
	roles := make([]string, 0, len(kind.Fields))
	for role := range kind.Fields {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	values := make(map[string]string, len(item.Fields))
	for _, f := range item.Fields {
		values[f.Label] = f.Value
	}
	creds := make(Creds, len(roles))
	labels := make([]string, 0, len(roles))
	for _, role := range roles {
		labels = append(labels, kind.Fields[role])
	}
	field := strings.Join(labels, ",")
	for _, role := range roles {
		label := kind.Fields[role]
		v, ok := values[label]
		if !ok {
			return missing(t, item, field, "field not found: "+label)
		}
		if v == "" {
			return missing(t, item, field, "field is empty: "+label)
		}
		creds[role] = Secret(v)
	}
	out := c.Check(ctx, creds)
	return Result{Name: t.Name, Item: item.Title, Field: field, Status: out.Status, Detail: out.Detail}
}

func missing(t Target, item *op.Item, field, detail string) Result {
	return Result{Name: t.Name, Item: item.Title, Field: field, Status: StatusMissing, Detail: detail}
}

// ExitCode is 1 if any credential is dead or absent, 2 if the only problems
// are inconclusive probes, else 0.
func ExitCode(results []Result) int {
	code := 0
	for _, r := range results {
		switch r.Status {
		case StatusFail, StatusMissing:
			return 1
		case StatusError:
			code = 2
		}
	}
	return code
}

func WriteTable(w io.Writer, results []Result) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tITEM\tFIELD\tSTATUS\tDETAIL")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, dash(r.Item), dash(r.Field), r.Status, r.Detail)
	}
	tw.Flush()
}

func WriteJSON(w io.Writer, results []Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
