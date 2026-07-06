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

func (c *Client) findVariable(ctx context.Context, wsID, key string) (string, error) {
	list, err := c.tfe.Variables.List(ctx, wsID, &tfe.VariableListOptions{})
	if err != nil {
		return "", fmt.Errorf("list variables for workspace %q: %w", wsID, err)
	}
	for _, v := range list.Items {
		if v.Key == key {
			return v.ID, nil
		}
	}
	return "", nil
}
