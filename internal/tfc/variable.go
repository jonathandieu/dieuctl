package tfc

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-tfe"
)

// UpsertVariable creates or updates a workspace variable. Sensitive variables
// are write-only in TFC — we always overwrite rather than check for drift.
func (c *Client) UpsertVariable(ctx context.Context, wsID, key, value string, sensitive bool) error {
	existing, err := c.findVariable(ctx, wsID, key)
	if err != nil {
		return err
	}

	category := tfe.CategoryTerraform

	if existing != "" {
		_, err := c.tfe.Variables.Update(ctx, wsID, existing, tfe.VariableUpdateOptions{
			Value:     tfe.String(value),
			Sensitive: tfe.Bool(sensitive),
		})
		if err != nil {
			return fmt.Errorf("update variable %q: %w", key, err)
		}
		return nil
	}

	_, err = c.tfe.Variables.Create(ctx, wsID, tfe.VariableCreateOptions{
		Key:       tfe.String(key),
		Value:     tfe.String(value),
		Category:  &category,
		Sensitive: tfe.Bool(sensitive),
	})
	if err != nil {
		return fmt.Errorf("create variable %q: %w", key, err)
	}
	return nil
}

// findVariable returns the ID of the variable matching key in the given
// workspace, or "" if none exists. Paginates through all variable pages,
// not just the first, so it doesn't miss the target on workspaces with many
// variables (which would otherwise cause UpsertVariable to create a
// duplicate instead of updating the existing one).
func (c *Client) findVariable(ctx context.Context, wsID, key string) (string, error) {
	opts := &tfe.VariableListOptions{ListOptions: tfe.ListOptions{PageSize: 100}}
	for {
		page, err := c.tfe.Variables.List(ctx, wsID, opts)
		if err != nil {
			return "", fmt.Errorf("list variables for workspace %q: %w", wsID, err)
		}
		for _, v := range page.Items {
			if v.Key == key {
				return v.ID, nil
			}
		}
		if page.CurrentPage >= page.TotalPages {
			return "", nil
		}
		opts.PageNumber = page.NextPage
	}
}
