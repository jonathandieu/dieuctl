package machine

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	skipMackup bool
	skipBrew   bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Bootstrap a new Mac from the jonathandieu/dotfiles repo",
	Long: `Full new-machine setup from the private jonathandieu/dotfiles repo.

Steps:
  1. dieuctl auth login   (1Password / Touch ID — must come first, repo is private)
  2. Install Homebrew (if missing) + brew update
  3. Clone github.com/jonathandieu/dotfiles → ~/dotfiles  (GitHub PAT from 1Password)
  4. brew bundle install --file=~/dotfiles/Brewfile  (includes doctl, kubectl, terraform, etc.)
  5. mackup restore        (symlinks dotfiles from iCloud/Dropbox)
  6. ~/dotfiles/macos/defaults.sh  (macOS system preferences)
  7. Set default shell

Use --skip-mackup to skip step 5 (no cloud storage access).
Use --skip-brew   to skip steps 2 and 4 (Homebrew already set up).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("machine setup — not yet implemented")
		fmt.Println("tracked in: dieuctl/docs/architecture.md § Machine setup flow")
		return nil
	},
}

func init() {
	setupCmd.Flags().BoolVar(&skipMackup, "skip-mackup", false, "Skip mackup restore (no cloud storage access)")
	setupCmd.Flags().BoolVar(&skipBrew, "skip-brew", false, "Skip Homebrew install and bundle (already set up)")
}
