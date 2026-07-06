package tfc

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-tfe"
)

// GetOrCreateWorkspace returns the named workspace, creating it in local
// execution mode if it doesn't already exist.
func (c *Client) GetOrCreateWorkspace(ctx context.Context, name string) (*tfe.Workspace, error) {
	ws, err := c.tfe.Workspaces.Read(ctx, c.org, name)
	if err == nil {
		return ws, nil
	}
	execMode := "local"
	ws, err = c.tfe.Workspaces.Create(ctx, c.org, tfe.WorkspaceCreateOptions{
		Name:          tfe.String(name),
		ExecutionMode: &execMode,
	})
	if err != nil {
		return nil, fmt.Errorf("create workspace %q: %w", name, err)
	}
	return ws, nil
}

// GetWorkspace fetches a workspace by name.
func (c *Client) GetWorkspace(ctx context.Context, name string) (*tfe.Workspace, error) {
	ws, err := c.tfe.Workspaces.Read(ctx, c.org, name)
	if err != nil {
		return nil, fmt.Errorf("get workspace %q: %w", name, err)
	}
	return ws, nil
}

// DeleteWorkspace deletes a workspace by name.
func (c *Client) DeleteWorkspace(ctx context.Context, name string) error {
	if err := c.tfe.Workspaces.Delete(ctx, c.org, name); err != nil {
		return fmt.Errorf("delete workspace %q: %w", name, err)
	}
	return nil
}

// ListWorkspaces returns all workspaces in the organisation.
func (c *Client) ListWorkspaces(ctx context.Context) ([]*tfe.Workspace, error) {
	var all []*tfe.Workspace
	opts := &tfe.WorkspaceListOptions{ListOptions: tfe.ListOptions{PageSize: 100}}
	for {
		page, err := c.tfe.Workspaces.List(ctx, c.org, opts)
		if err != nil {
			return nil, fmt.Errorf("list workspaces: %w", err)
		}
		all = append(all, page.Items...)
		if page.CurrentPage >= page.TotalPages {
			break
		}
		opts.PageNumber = page.NextPage
	}
	return all, nil
}

// ApplyVariableSet assigns a named variable set to a workspace.
func (c *Client) ApplyVariableSet(ctx context.Context, varSetName string, ws *tfe.Workspace) error {
	// Look up variable set by name
	varSets, err := c.tfe.VariableSets.List(ctx, c.org, &tfe.VariableSetListOptions{})
	if err != nil {
		return fmt.Errorf("list variable sets: %w", err)
	}
	var vsID string
	for _, vs := range varSets.Items {
		if vs.Name == varSetName {
			vsID = vs.ID
			break
		}
	}
	if vsID == "" {
		return fmt.Errorf("variable set %q not found in org %q", varSetName, c.org)
	}

	err = c.tfe.VariableSets.ApplyToWorkspaces(ctx, vsID, &tfe.VariableSetApplyToWorkspacesOptions{
		Workspaces: []*tfe.Workspace{ws},
	})
	if err != nil {
		return fmt.Errorf("apply variable set %q to workspace %q: %w", varSetName, ws.Name, err)
	}
	return nil
}
