package auth

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage 1Password authentication",
}

func init() {
	Cmd.AddCommand(loginCmd, statusCmd, logoutCmd)
}
