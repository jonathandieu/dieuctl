package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var accountFlag string

var Cmd = &cobra.Command{
	Use:   "exec -- <command> [args...]",
	Short: "Run a command with secrets injected from 1Password",
	Long: `Run any command with the full secret environment injected from 1Password.

Context detection: when --account is not set, the account is inferred from
the current directory (e.g. clusters/dieubernetes-do-platform-nyc3/ → "platform").

Examples:
  dieuctl exec -- terraform plan
  dieuctl exec -- doctl kubernetes cluster list
  dieuctl exec --account spare -- terraform apply`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}

		account := accountFlag
		if account == "" {
			account = inferAccount(cfg)
		}

		env, err := buildEnv(ctx, cfg, resolver, account)
		if err != nil {
			return err
		}

		// Find and exec the command
		binPath, err := exec.LookPath(args[0])
		if err != nil {
			return fmt.Errorf("command %q not found: %w", args[0], err)
		}

		c := exec.CommandContext(ctx, binPath, args[1:]...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Env = mergeEnv(os.Environ(), env)

		if err := c.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return err
		}
		return nil
	},
}

func init() {
	Cmd.Flags().StringVar(&accountFlag, "account", "", "DigitalOcean account alias (overrides context detection)")
}

// inferAccount parses the current working directory to determine the DO account.
// clusters/dieubernetes-do-{account}-{region}/ → account
func inferAccount(cfg *config.Config) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Walk up until we find a path component starting with "dieubernetes-do-"
	parts := strings.Split(filepath.ToSlash(cwd), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "dieubernetes-do-") {
			// dieubernetes-do-{account}-{region}
			segments := strings.SplitN(strings.TrimPrefix(part, "dieubernetes-do-"), "-", 2)
			if len(segments) == 2 {
				return segments[0]
			}
		}
	}
	return ""
}

// buildEnv assembles the secret environment map for the given account.
func buildEnv(ctx context.Context, cfg *config.Config, resolver *op.Resolver, account string) (map[string]string, error) {
	refs := map[string]string{
		"TF_TOKEN_app_terraform_io": cfg.TFC.Token,
		"CLOUDFLARE_API_TOKEN":      cfg.Cloudflare.APIToken,
	}

	if account != "" {
		acct, err := cfg.DOAccount(account)
		if err != nil {
			return nil, err
		}
		refs["DIGITALOCEAN_TOKEN"] = acct.Token
	}

	resolved, err := resolver.ResolveMap(ctx, refs)
	if err != nil {
		return nil, fmt.Errorf("resolve secrets: %w", err)
	}
	return resolved, nil
}

// mergeEnv overlays overrides on top of base, returning a new slice.
func mergeEnv(base []string, overrides map[string]string) []string {
	overrideSet := make(map[string]string, len(overrides))
	for k, v := range overrides {
		overrideSet[k] = v
	}

	result := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key := strings.SplitN(kv, "=", 2)[0]
		if _, ok := overrideSet[key]; !ok {
			result = append(result, kv)
		}
	}
	for k, v := range overrideSet {
		result = append(result, k+"="+v)
	}
	return result
}
