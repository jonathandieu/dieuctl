package auth

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with 1Password (Touch ID)",
	Long: `Start a 1Password CLI session. Prompts for Touch ID or password.

After login, dieuctl resolves op:// references using the active session.
The session lasts 30 minutes; run again to extend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opPath, err := exec.LookPath("op")
		if err != nil {
			return fmt.Errorf("`op` CLI not found — install from https://1password.com/downloads/command-line/")
		}

		// op signin writes session env vars to stdout; we run it so the user
		// sees the Touch ID prompt. We can't set parent shell env vars from here,
		// so we print the eval hint.
		signinCmd := exec.Command(opPath, "signin")
		signinCmd.Stdin = os.Stdin
		signinCmd.Stdout = os.Stdout
		signinCmd.Stderr = os.Stderr
		if err := signinCmd.Run(); err != nil {
			return fmt.Errorf("op signin: %w", err)
		}

		fmt.Println()
		fmt.Println("Session started. To make secrets available in your shell:")
		fmt.Println()
		fmt.Println("  eval $(op signin)")
		fmt.Println()
		fmt.Println("Or use `dieuctl exec -- <command>` to inject secrets into any subprocess without modifying your shell environment.")
		return nil
	},
}
