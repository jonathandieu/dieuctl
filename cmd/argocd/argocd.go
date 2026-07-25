package argocd

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "argocd",
	Short: "Manage ArgoCD cluster registrations",
}

func init() {
	Cmd.AddCommand(registerCmd, syncWaitCmd)
}
