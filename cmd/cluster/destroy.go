package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jonathandieu/dieuctl/internal/cloudflare"
	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/gitops"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/terraform"
	"github.com/jonathandieu/dieuctl/internal/tfc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var destroyForce bool

var destroyCmd = &cobra.Command{
	Use:   "destroy <name>",
	Short: "Tear down a cluster and clean up all associated resources",
	Long: `Destroy a DOKS cluster and remove all associated resources:
  - Terraform destroy (with one automatic retry for async VPC deletion)
  - Cloudflare A record deleted
  - Kubeconfig removed from ~/.kube/
  - Terraform cluster directory removed and committed
  - Dieubernetes cluster directory removed and committed
  - TFC workspace deleted`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		if !destroyForce {
			fmt.Printf("This will permanently destroy cluster %q and all its resources.\n", name)
			fmt.Print("Type the cluster name to confirm: ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			if strings.TrimSpace(scanner.Text()) != name {
				return fmt.Errorf("confirmation did not match — aborting")
			}
		}

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}

		tfcToken, err := resolver.Resolve(ctx, cfg.TFC.Token)
		if err != nil {
			return fmt.Errorf("resolve TFC token: %w", err)
		}
		cfToken, err := resolver.Resolve(ctx, cfg.Cloudflare.APIToken)
		if err != nil {
			return fmt.Errorf("resolve Cloudflare token: %w", err)
		}
		cfZoneID, err := resolver.Resolve(ctx, cfg.Cloudflare.ZoneID)
		if err != nil {
			return fmt.Errorf("resolve Cloudflare zone ID: %w", err)
		}

		// Resolve account from cluster name (middle segment: dieubernetes-do-{account}-{region})
		parts := strings.Split(name, "-")
		var accountName string
		if len(parts) >= 3 {
			accountName = parts[2]
		}
		acctCfg, err := cfg.DOAccount(accountName)
		if err != nil {
			return fmt.Errorf("resolve account from cluster name %q: %w", name, err)
		}
		doToken, err := resolver.Resolve(ctx, acctCfg.Token)
		if err != nil {
			return fmt.Errorf("resolve DO token: %w", err)
		}

		tfcClient, err := tfc.New(ctx, tfcToken, cfg.TFC.Organization)
		if err != nil {
			return err
		}

		clusterDir := filepath.Join(cfg.TerraformRepo, "clusters", name)
		if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
			return fmt.Errorf("cluster directory not found: %s", clusterDir)
		}

		tfEnv := map[string]string{
			"DIGITALOCEAN_TOKEN":        doToken,
			"TF_TOKEN_app_terraform_io": tfcToken,
		}

		// 1. Terraform destroy — retry once on failure to handle DO's async node
		// cleanup: cluster nodes are deleted asynchronously, so the next destroy
		// run succeeds once they're gone.
		fmt.Println("→ running terraform destroy (this takes ~5 minutes)...")
		err = terraform.Run(ctx, clusterDir, tfEnv, "destroy", "-auto-approve", "-input=false")
		if err != nil {
			fmt.Printf("  destroy failed: %v\n", err)
			fmt.Println("  waiting 90s for async DO resource cleanup then retrying...")
			time.Sleep(90 * time.Second)
			err = terraform.Run(ctx, clusterDir, tfEnv, "destroy", "-auto-approve", "-input=false")
			if err != nil {
				return fmt.Errorf("destroy failed after retry: %w", err)
			}
		}
		fmt.Println("  ✓ infrastructure destroyed")

		// 2. Delete Cloudflare A record
		fmt.Println("→ deleting Cloudflare A record...")
		cfClient := cloudflare.New(cfToken, cfZoneID)
		recordName := fmt.Sprintf("%s.%s", name, cfg.Cloudflare.Domain)
		if err := cfClient.DeleteRecordByName(ctx, recordName, "A"); err != nil {
			fmt.Printf("  warning: %v (continuing)\n", err)
		} else {
			fmt.Printf("  ✓ deleted %s\n", recordName)
		}

		// 3. Remove kubeconfig
		kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", name+".yaml")
		if err := os.Remove(kubeconfigPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("  warning: remove kubeconfig: %v (continuing)\n", err)
		} else {
			fmt.Printf("  ✓ removed %s\n", kubeconfigPath)
		}

		// 4. Remove terraform cluster dir and commit
		fmt.Println("→ removing terraform cluster directory...")
		if err := os.RemoveAll(clusterDir); err != nil {
			return fmt.Errorf("remove terraform cluster dir: %w", err)
		}
		tfRepo, err := gitops.Open(cfg.TerraformRepo)
		if err != nil {
			return fmt.Errorf("open terraform repo: %w", err)
		}
		if err := tfRepo.AddAll(); err != nil {
			return fmt.Errorf("stage terraform removal: %w", err)
		}
		if err := tfRepo.Commit(fmt.Sprintf("feat: remove cluster %s", name)); err != nil {
			return fmt.Errorf("commit terraform removal: %w", err)
		}
		fmt.Println("  ✓ terraform cluster directory removed and committed")

		// 5. Remove dieubernetes cluster dir and commit
		fmt.Println("→ removing dieubernetes cluster directory...")
		gitopsClusterDir := filepath.Join(cfg.DieubernetesRepo, "clusters", name)
		if err := os.RemoveAll(gitopsClusterDir); err != nil {
			return fmt.Errorf("remove dieubernetes cluster dir: %w", err)
		}
		gitopsRepo, err := gitops.Open(cfg.DieubernetesRepo)
		if err != nil {
			return fmt.Errorf("open dieubernetes repo: %w", err)
		}
		if err := gitopsRepo.AddAll(); err != nil {
			return fmt.Errorf("stage dieubernetes removal: %w", err)
		}
		if err := gitopsRepo.Commit(fmt.Sprintf("feat: deregister cluster %s", name)); err != nil {
			return fmt.Errorf("commit dieubernetes removal: %w", err)
		}
		fmt.Println("  ✓ dieubernetes cluster directory removed and committed")

		// 6. Delete TFC workspace (best-effort — workspace may not exist if state was never pushed)
		fmt.Println("→ deleting TFC workspace...")
		if err := tfcClient.DeleteWorkspace(ctx, name); err != nil {
			fmt.Printf("  warning: %v (continuing)\n", err)
		} else {
			fmt.Printf("  ✓ workspace %q deleted\n", name)
		}

		fmt.Println()
		fmt.Printf("cluster %q destroyed\n", name)
		fmt.Println("push both repos to sync the deletions upstream")
		return nil
	},
}

func init() {
	destroyCmd.Flags().BoolVar(&destroyForce, "force", false, "Skip confirmation prompt")
}
