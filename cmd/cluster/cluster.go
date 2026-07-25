package cluster

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage DOKS clusters",
}

func init() {
	Cmd.AddCommand(createCmd, listCmd, statusCmd, destroyCmd)
}
