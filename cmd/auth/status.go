package auth

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show 1Password session state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN"); token != "" {
			fmt.Println("auth: service account token (OP_SERVICE_ACCOUNT_TOKEN)")
			return nil
		}

		opPath, err := exec.LookPath("op")
		if err != nil {
			return fmt.Errorf("`op` CLI not found")
		}

		out, err := exec.Command(opPath, "whoami").Output()
		if err != nil {
			fmt.Println("not authenticated — run: dieuctl auth login")
			return nil
		}
		fmt.Print(string(out))
		return nil
	},
}
