package emoji

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "emoji",
	Short: "Manage a portable archive of your Slack custom emoji",
}

func init() {
	Cmd.AddCommand(saveCmd)
}
