package machine

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "machine",
	Short: "Set up and maintain this Mac",
}

func init() {
	Cmd.AddCommand(setupCmd, syncCmd)
}
