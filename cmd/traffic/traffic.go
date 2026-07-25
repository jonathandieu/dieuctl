package traffic

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "traffic",
	Short: "Manage traffic routing between clusters",
}

func init() {
	Cmd.AddCommand(switchCmd, trafficStatusCmd, rollbackCmd)
}
