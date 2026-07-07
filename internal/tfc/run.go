package tfc

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-tfe"
)

// ReadOutputs returns the state outputs for a workspace as a string map.
func (c *Client) ReadOutputs(ctx context.Context, wsID string) (map[string]string, error) {
	sv, err := c.tfe.StateVersions.ReadCurrentWithOptions(ctx, wsID, &tfe.StateVersionCurrentOptions{
		Include: []tfe.StateVersionIncludeOpt{tfe.SVoutputs},
	})
	if err != nil {
		return nil, fmt.Errorf("read state version: %w", err)
	}

	out := make(map[string]string, len(sv.Outputs))
	for _, o := range sv.Outputs {
		// Value is interface{}, coerce to string
		out[o.Name] = fmt.Sprintf("%v", o.Value)
	}
	return out, nil
}
