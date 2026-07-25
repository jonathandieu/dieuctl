package cluster

import (
	"context"
	"fmt"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/do"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/tfc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var kubeconfigAccount string

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig <cluster-name>",
	Short: "Fetch and save kubeconfig for a cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}

		acctCfg, err := cfg.DOAccount(kubeconfigAccount)
		if err != nil {
			return err
		}

		tfcToken, err := resolver.Resolve(ctx, cfg.TFC.Token)
		if err != nil {
			return fmt.Errorf("resolve TFC token: %w", err)
		}
		doToken, err := resolver.Resolve(ctx, acctCfg.Token)
		if err != nil {
			return fmt.Errorf("resolve DO token: %w", err)
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

		path, err := fetchAndSaveKubeconfig(ctx, doToken, clusterID, name)
		if err != nil {
			return err
		}

		fmt.Printf("kubeconfig saved to %s\n", path)
		fmt.Printf("context merged into ~/.kube/config\n")
		return nil
	},
}

// fetchAndSaveKubeconfig fetches a fresh kubeconfig from the DO API and writes it to ~/.kube/<name>.yaml.
func fetchAndSaveKubeconfig(ctx context.Context, doToken, clusterID, clusterName string) (string, error) {
	kubeconfig, err := do.New(doToken).FetchKubeconfig(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("fetch kubeconfig: %w", err)
	}
	path, err := do.SaveKubeconfig(clusterName, kubeconfig)
	if err != nil {
		return "", fmt.Errorf("save kubeconfig: %w", err)
	}
	return path, nil
}

func init() {
	kubeconfigCmd.Flags().StringVar(&kubeconfigAccount, "account", "", "DigitalOcean account alias (required)")
	_ = kubeconfigCmd.MarkFlagRequired("account")
	Cmd.AddCommand(kubeconfigCmd)
}
