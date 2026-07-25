package machine

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull latest dotfiles + brew upgrade + mackup backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("machine sync — not yet implemented")
		return nil
	},
}
