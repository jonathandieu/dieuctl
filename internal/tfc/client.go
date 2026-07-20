package tfc

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-tfe"
)

type Client struct {
	tfe *tfe.Client
	org string
}

func New(ctx context.Context, token, org string) (*Client, error) {
	cfg := &tfe.Config{Token: token}
	c, err := tfe.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("init TFC client: %w", err)
	}
	return &Client{tfe: c, org: org}, nil
}
