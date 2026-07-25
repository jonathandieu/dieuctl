package cluster

import (
	"context"
	"fmt"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/kubectl"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/tfc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var bootstrapAccount string

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap <cluster-name>",
	Short: "Create or rotate the 1Password service account secret on an existing cluster",
	Long: `Creates or updates the op-service-account-token secret in the external-secrets
namespace. Safe to re-run — uses kubectl apply so it creates or updates idempotently.

Use this when:
  - Bootstrapping a cluster that was provisioned before dieuctl managed secrets
  - Rotating the service account token after updating it in 1Password

Example:
  dieuctl cluster bootstrap dieubernetes-platform-do-atl1 --account platform`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if cfg.OnePassword.ServiceAccountToken == "" {
			return fmt.Errorf("onepassword.service_account_token is not set in config — create a 1Password service account at 1password.com and add the token reference to ~/.config/dieuctl/config.yaml")
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}

		acctCfg, err := cfg.DOAccount(bootstrapAccount)
		if err != nil {
			return err
		}

		doToken, err := resolver.Resolve(ctx, acctCfg.Token)
		if err != nil {
			return fmt.Errorf("resolve DO token: %w", err)
		}
		saToken, err := resolver.Resolve(ctx, cfg.OnePassword.ServiceAccountToken)
		if err != nil {
			return fmt.Errorf("resolve 1Password service account token: %w", err)
		}

		tfcToken, err := resolver.Resolve(ctx, cfg.TFC.Token)
		if err != nil {
			return fmt.Errorf("resolve TFC token: %w", err)
		}
		tfcClient, err := tfc.New(ctx, tfcToken, cfg.TFC.Organization)
		if err != nil {
			return err
		}
		ws, err := tfcClient.GetWorkspace(ctx, name)
		if err != nil {
			return fmt.Errorf("get TFC workspace: %w", err)
		}
		outputs, err := tfcClient.ReadOutputs(ctx, ws.ID)
		if err != nil {
			return fmt.Errorf("read TFC outputs: %w", err)
		}
		clusterID := outputs["cluster_id"]
		if clusterID == "" {
			return fmt.Errorf("cluster_id not found in TFC outputs for workspace %q", name)
		}

		kubeconfigPath, err := fetchAndSaveKubeconfig(ctx, doToken, clusterID, name)
		if err != nil {
			return fmt.Errorf("fetch kubeconfig: %w", err)
		}

		kube, err := kubectl.New(kubeconfigPath)
		if err != nil {
			return err
		}

		fmt.Printf("→ bootstrapping op-service-account-token on %s...\n", name)
		if err := bootstrapOPServiceAccountSecret(ctx, kube, saToken); err != nil {
			return fmt.Errorf("bootstrap secret: %w", err)
		}
		fmt.Println("  ✓ op-service-account-token created/updated in external-secrets")
		return nil
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&bootstrapAccount, "account", "", "DigitalOcean account alias (required)")
	_ = bootstrapCmd.MarkFlagRequired("account")
	Cmd.AddCommand(bootstrapCmd)
}
