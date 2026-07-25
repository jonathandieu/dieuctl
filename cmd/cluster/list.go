package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/tfc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return err
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}

		tfcToken, err := resolver.Resolve(ctx, cfg.TFC.Token)
		if err != nil {
			return err
		}

		tfcClient, err := tfc.New(ctx, tfcToken, cfg.TFC.Organization)
		if err != nil {
			return err
		}

		workspaces, err := tfcClient.ListWorkspaces(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("%-50s  %s\n", "CLUSTER", "STATUS")
		fmt.Printf("%-50s  %s\n", strings.Repeat("-", 50), strings.Repeat("-", 10))

		for _, ws := range workspaces {
			if !strings.HasPrefix(ws.Name, "dieubernetes-") {
				continue
			}
			fmt.Printf("%-50s  %s\n", ws.Name, ws.ID)
		}
		return nil
	},
}
