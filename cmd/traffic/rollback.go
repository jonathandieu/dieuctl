package traffic

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Flip traffic back to the previous cluster (~10s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("traffic rollback — not yet implemented")
		fmt.Println("planned: read previous_cluster from config → update active_cluster → trigger TFC apply")
		return nil
	},
}
