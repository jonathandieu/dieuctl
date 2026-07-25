package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show cluster node health and ArgoCD sync state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("status for %q — not yet implemented\n", args[0])
		fmt.Println("planned: node ready counts + ArgoCD app health summary")
		return nil
	},
}
