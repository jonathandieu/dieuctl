package traffic

import (
	"fmt"

	"github.com/spf13/cobra"
)

var trafficStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which cluster is currently serving traffic",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("traffic status — not yet implemented")
		fmt.Println("planned: read active_cluster from Cloudflare Workers KV + show CNAME targets")
		return nil
	},
}
