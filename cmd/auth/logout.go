package auth

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "End 1Password session",
	RunE: func(cmd *cobra.Command, args []string) error {
		opPath, err := exec.LookPath("op")
		if err != nil {
			return fmt.Errorf("`op` CLI not found")
		}
		out, err := exec.Command(opPath, "signout").CombinedOutput()
		if err != nil {
			return fmt.Errorf("op signout: %s", string(out))
		}
		fmt.Println("signed out")
		return nil
	},
}
