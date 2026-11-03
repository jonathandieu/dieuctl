package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/secrets"
)

var (
	secretsJSON bool
	secretsList bool
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Credential helpers",
}

var secretsCheckCmd = &cobra.Command{
	Use:   "check [name...]",
	Short: "Check that 1Password API credentials still authenticate",
	Long: `Reads each credential from 1Password and makes one cheap read-only request to
the vendor to confirm it still authenticates. Secrets are never printed.

Items are looked up by title in the configured vault (onepassword.vault,
default "1Password"). Providers with several keys in one item (for example
Anthropic) get one row per concealed field. DigitalOcean is split by account
(personal, work), each with a Spaces access key pair.

Exit status: 0 all pass, 1 any FAIL or MISSING, 2 only inconclusive ERRORs.
Refuses to run when CI is set; run it locally.

Extra or remapped credentials go under secrets.targets in the config file:

  secrets:
    spaces_region: nyc3
    targets:
      - name: digitalocean.work
        checker: digitalocean
        item: DigitalOcean PAT - free

Checkers: anthropic, openai, digitalocean, digitalocean-spaces, cloudflare, github, tfc.`,
	SilenceUsage: true,
	RunE:         runSecretsCheck,
}

func init() {
	secretsCheckCmd.Flags().BoolVar(&secretsJSON, "json", false, "emit JSON instead of a table")
	secretsCheckCmd.Flags().BoolVar(&secretsList, "list", false, "list known targets without contacting 1Password or vendors")
	secretsCmd.AddCommand(secretsCheckCmd)
	rootCmd.AddCommand(secretsCmd)
}

func runSecretsCheck(cmd *cobra.Command, args []string) error {
	var cfg secrets.Config
	if err := viper.UnmarshalKey("secrets", &cfg); err != nil {
		return fmt.Errorf("read secrets config: %w", err)
	}
	kinds := secrets.Kinds(secrets.NewHTTPClient())

	all, err := secrets.BuildTargets(cfg, kinds)
	if err != nil {
		return err
	}
	targets, err := secrets.Select(all, args)
	if err != nil {
		return err
	}

	if secretsList {
		for _, t := range targets {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", t.Name, t.Kind, strings.Join(t.Items, " | "))
		}
		return nil
	}

	if inCI() {
		return fmt.Errorf("secrets check holds live API keys and only runs locally, not in CI")
	}
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("op CLI not found, install it from https://1password.com/downloads/command-line/: %w", err)
	}

	vault := viper.GetString("onepassword.vault")
	if vault == "" {
		vault = "1Password"
	}
	runner := secrets.Runner{Items: op.ItemReader{}, Vault: vault, Kinds: kinds}
	results := runner.Run(cmd.Context(), targets)

	if secretsJSON {
		if err := secrets.WriteJSON(cmd.OutOrStdout(), results); err != nil {
			return err
		}
	} else {
		secrets.WriteTable(cmd.OutOrStdout(), results)
	}
	if code := secrets.ExitCode(results); code != 0 {
		os.Exit(code)
	}
	return nil
}

func inCI() bool {
	ci := os.Getenv("CI")
	return (ci != "" && ci != "false" && ci != "0") || os.Getenv("GITHUB_ACTIONS") != ""
}
