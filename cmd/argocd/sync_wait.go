package argocd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncWaitCmd = &cobra.Command{
	Use:   "sync-wait <cluster-name>",
	Short: "Block until all ArgoCD apps on a cluster are Healthy+Synced",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("argocd sync-wait %s — not yet implemented\n", args[0])
		fmt.Println("planned: poll ArgoCD API until all apps for this cluster destination are Health=Healthy Sync=Synced")
		return nil
	},
}
