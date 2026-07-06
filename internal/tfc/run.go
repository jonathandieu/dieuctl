package tfc

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-tfe"
)

// TriggerRun queues a run on the workspace and waits for it to reach a terminal
// state (applied or errored). Returns the run on success.
func (c *Client) TriggerRun(ctx context.Context, ws *tfe.Workspace, message string) (*tfe.Run, error) {
	run, err := c.tfe.Runs.Create(ctx, tfe.RunCreateOptions{
		Workspace: ws,
		Message:   tfe.String(message),
		AutoApply: tfe.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	return c.WaitForRun(ctx, run.ID)
}

// TriggerDestroyRun queues a destroy run and waits for it to reach a terminal state.
func (c *Client) TriggerDestroyRun(ctx context.Context, ws *tfe.Workspace, message string) (*tfe.Run, error) {
	run, err := c.tfe.Runs.Create(ctx, tfe.RunCreateOptions{
		Workspace: ws,
		Message:   tfe.String(message),
		AutoApply: tfe.Bool(true),
		IsDestroy: tfe.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("create destroy run: %w", err)
	}

	return c.WaitForRun(ctx, run.ID)
}

// WaitForRun polls until the run reaches a terminal state.
func (c *Client) WaitForRun(ctx context.Context, runID string) (*tfe.Run, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			run, err := c.tfe.Runs.Read(ctx, runID)
			if err != nil {
				return nil, fmt.Errorf("read run %q: %w", runID, err)
			}
			switch run.Status {
			case tfe.RunApplied:
				return run, nil
			case tfe.RunErrored, tfe.RunCanceled, tfe.RunDiscarded, tfe.RunPolicySoftFailed:
				return nil, fmt.Errorf("run %q ended with status %q", runID, run.Status)
			}
			// still pending/planning/applying — keep polling
		}
	}
}

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
		// Value is interface{} — coerce to string
		out[o.Name] = fmt.Sprintf("%v", o.Value)
	}
	return out, nil
}
