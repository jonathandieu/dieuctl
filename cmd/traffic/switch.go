package traffic

import (
	"fmt"

	"github.com/spf13/cobra"
)

var switchTo string

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch active traffic to a different cluster",
	Long: `Pre-flight checks → optional DB migration → Cloudflare CNAME flip.

Pre-flight (all must pass):
  - Target cluster nodes Ready
  - All ArgoCD apps Healthy+Synced on target
  - Smoke tests pass on cluster subdomain

Not yet implemented.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("traffic switch --to %s — not yet implemented\n", switchTo)
		return nil
	},
}

func init() {
	switchCmd.Flags().StringVar(&switchTo, "to", "", "Target cluster name (required)")
	_ = switchCmd.MarkFlagRequired("to")
}
