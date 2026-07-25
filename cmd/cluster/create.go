package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jonathandieu/dieuctl/internal/cloudflare"
	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/gitops"
	"github.com/jonathandieu/dieuctl/internal/kubectl"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/terraform"
	"github.com/jonathandieu/dieuctl/internal/tfc"
	"github.com/jonathandieu/dieuctl/templates"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

var (
	createName        string
	createTier        string
	createAccount     string
	createRegion      string
	createSize        string
	createProvider    string
	createAutoApprove bool
	createMinNodes    int
	createMaxNodes    int
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Provision a new cluster end-to-end",
	Long: `Create a new DOKS cluster and register it with the platform.

Steps:
  1. Generate Terraform files → commit + push terraform repo
  2. Create TFC workspace → assign variable set → trigger apply
  3. Create Cloudflare A record for cluster subdomain
  4. Generate dieubernetes cluster.yaml + envoy-gateway override → commit + push
  5. Register cluster with ArgoCD on platform cluster
  6. Wait for all ArgoCD apps to sync

Example:
  dieuctl cluster create --account spare --region ams3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

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

		acctCfg, err := cfg.DOAccount(createAccount)
		if err != nil {
			return err
		}

		name := createName
		if name == "" {
			name = deriveName(createTier, createProvider, createRegion)
		}
		fmt.Printf("creating cluster %q\n", name)

		// Resolve secrets
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

		// Resolve DO token — injected as env var into terraform subprocess
		doToken, err := resolver.Resolve(ctx, acctCfg.Token)
		if err != nil {
			return fmt.Errorf("resolve DO token: %w", err)
		}

		// 1. Generate Terraform files and commit
		fmt.Println("→ generating terraform files...")
		vars := gitops.ClusterVars{
			Name:     name,
			Provider: createProvider,
			Account:  createAccount,
			Region:   createRegion,
			NodeSize: createSize,
			MinNodes: createMinNodes,
			MaxNodes: createMaxNodes,
			Tier:     createTier,
		}

		clusterDir := filepath.Join(cfg.TerraformRepo, "clusters", name)
		if err := generateTerraformFiles(cfg.TerraformRepo, vars); err != nil {
			return fmt.Errorf("generate terraform files: %w", err)
		}
		fmt.Println("  ✓ terraform files generated")

		// 2. Ensure TFC workspace exists (GetOrCreate — idempotent)
		fmt.Println("→ ensuring TFC workspace...")
		tfcClient, err := tfc.New(ctx, tfcToken, cfg.TFC.Organization)
		if err != nil {
			return err
		}
		ws, err := tfcClient.GetOrCreateWorkspace(ctx, name)
		if err != nil {
			return err
		}
		fmt.Printf("  ✓ workspace %q ready\n", ws.Name)

		// 3. Build terraform env — secrets resolved from 1Password, never touch disk
		tfEnv := map[string]string{
			"DIGITALOCEAN_TOKEN":        doToken,
			"TF_TOKEN_app_terraform_io": tfcToken,
		}

		// Management (platform) clusters: bcrypt the ArgoCD password and pass as TF var
		if vars.Tier == "platform" {
			adminPassword, err := resolver.Resolve(ctx, cfg.ArgoCD.AdminPassword)
			if err != nil {
				return fmt.Errorf("resolve argocd admin password: %w", err)
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 10)
			if err != nil {
				return fmt.Errorf("bcrypt argocd password: %w", err)
			}
			tfEnv["TF_VAR_argocd_admin_password_bcrypt"] = string(hash)
		}

		// 4. terraform init + apply
		fmt.Println("→ running terraform init...")
		if err := terraform.Run(ctx, clusterDir, tfEnv, "init", "-input=false"); err != nil {
			return fmt.Errorf("terraform init: %w", err)
		}
		fmt.Println("→ running terraform apply (this takes ~8 minutes)...")
		if err := terraform.Run(ctx, clusterDir, tfEnv, "apply", "-auto-approve", "-input=false"); err != nil {
			return fmt.Errorf("terraform apply: %w", err)
		}
		fmt.Println("  ✓ infrastructure provisioned")

		// 5. Read outputs
		fmt.Println("→ reading cluster outputs...")
		outputs, err := terraform.Outputs(ctx, clusterDir, tfEnv)
		if err != nil {
			return fmt.Errorf("read outputs: %w", err)
		}
		lbIP := outputs["lb_ip"]
		lbID := outputs["lb_id"]
		clusterID := outputs["cluster_id"]
		if lbIP == "" || lbID == "" || clusterID == "" {
			return fmt.Errorf("expected lb_ip, lb_id, cluster_id in outputs, got: %v", outputs)
		}
		fmt.Printf("  ✓ lb_ip=%s lb_id=%s\n", lbIP, lbID)

		// 6. Commit terraform files
		tfRepo, err := gitops.Open(cfg.TerraformRepo)
		if err != nil {
			return fmt.Errorf("open terraform repo: %w", err)
		}
		if err := tfRepo.Add(filepath.Join("clusters", name)); err != nil {
			return fmt.Errorf("stage terraform files: %w", err)
		}
		if err := tfRepo.Commit(fmt.Sprintf("feat: add cluster %s", name)); err != nil {
			return fmt.Errorf("commit terraform files: %w", err)
		}
		fmt.Println("  ✓ terraform files committed")

		// 7. Save kubeconfig
		fmt.Println("→ saving kubeconfig...")
		kubeconfigPath, err := fetchAndSaveKubeconfig(ctx, doToken, clusterID, name)
		if err != nil {
			return fmt.Errorf("save kubeconfig: %w", err)
		}
		fmt.Printf("  ✓ kubeconfig saved to %s\n", kubeconfigPath)

		// 7b. Bootstrap 1Password service account secret (all clusters).
		// ESO needs this secret before it can reconcile the ClusterSecretStore.
		fmt.Println("→ bootstrapping 1Password service account secret...")
		saToken, err := resolver.Resolve(ctx, cfg.OnePassword.ServiceAccountToken)
		if err != nil {
			return fmt.Errorf("resolve 1Password service account token: %w", err)
		}
		kube, err := kubectl.New(kubeconfigPath)
		if err != nil {
			return err
		}
		if err := bootstrapOPServiceAccountSecret(ctx, kube, saToken); err != nil {
			return fmt.Errorf("bootstrap 1Password service account secret: %w", err)
		}
		fmt.Println("  ✓ op-service-account-token created in external-secrets")

		// 8. Cloudflare DNS records
		fmt.Println("→ creating Cloudflare DNS record...")
		cfClient := cloudflare.New(cfToken, cfZoneID)
		recordName := fmt.Sprintf("%s.%s", name, cfg.Cloudflare.Domain)
		_, err = cfClient.UpsertARecord(ctx, recordName, lbIP, false)
		if err != nil {
			return fmt.Errorf("create DNS record: %w", err)
		}
		fmt.Printf("  ✓ %s → %s\n", recordName, lbIP)

		// Platform clusters: update shared/cloudflare to point apex + argocd at this cluster
		if vars.Tier == "platform" {
			fmt.Println("→ updating shared Cloudflare workspace...")
			cfDir := filepath.Join(cfg.TerraformRepo, "shared", "cloudflare")
			if err := updatePrimaryClusterWorkspace(cfDir, name); err != nil {
				return fmt.Errorf("update cloudflare tfvars: %w", err)
			}
			cfEnv := map[string]string{
				"CLOUDFLARE_API_TOKEN":       cfToken,
				"TF_TOKEN_app_terraform_io":  tfcToken,
			}
			if err := terraform.Run(ctx, cfDir, cfEnv, "apply", "-auto-approve", "-input=false"); err != nil {
				return fmt.Errorf("cloudflare terraform apply: %w", err)
			}
			fmt.Printf("  ✓ apex + argocd.%s → %s\n", cfg.Cloudflare.Domain, lbIP)
		}

		// 9. Generate dieubernetes files and commit
		fmt.Println("→ generating dieubernetes cluster files...")
		vars.LBID = lbID
		vars.LBIP = lbIP

		if err := generateDieubernetesFiles(cfg.DieubernetesRepo, vars); err != nil {
			return fmt.Errorf("generate dieubernetes files: %w", err)
		}

		gitopsRepo, err := gitops.Open(cfg.DieubernetesRepo)
		if err != nil {
			return fmt.Errorf("open dieubernetes repo: %w", err)
		}
		if err := gitopsRepo.Add(filepath.Join("clusters", name)); err != nil {
			return fmt.Errorf("stage dieubernetes files: %w", err)
		}
		if err := gitopsRepo.Commit(fmt.Sprintf("feat: register cluster %s", name)); err != nil {
			return fmt.Errorf("commit dieubernetes files: %w", err)
		}
		fmt.Println("  ✓ dieubernetes cluster files committed")

		fmt.Println()
		fmt.Printf("cluster %q provisioned successfully!\n", name)
		fmt.Println()
		fmt.Println("next steps:")
		fmt.Println("  push both repos, then:")
		if vars.Tier == "platform" {
			fmt.Println("  ArgoCD will sync automatically once dieubernetes is pushed")
		} else {
			fmt.Printf("  dieuctl argocd register %s\n", name)
			fmt.Printf("  dieuctl argocd sync-wait %s\n", name)
		}

		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createName, "name", "", "Override derived cluster name")
	createCmd.Flags().StringVar(&createTier, "tier", "platform", "Cluster tier: platform | stage | prod")
	createCmd.Flags().StringVar(&createAccount, "account", "", "DigitalOcean account alias (required)")
	createCmd.Flags().StringVar(&createRegion, "region", "", "DigitalOcean region slug, e.g. nyc3 (required)")
	createCmd.Flags().StringVar(&createSize, "size", "s-2vcpu-8gb-amd", "Node size slug")
	createCmd.Flags().StringVar(&createProvider, "provider", "do", "Cloud provider (do)")
	createCmd.Flags().BoolVar(&createAutoApprove, "auto-approve", false, "Skip TFC plan confirmation")
	createCmd.Flags().IntVar(&createMinNodes, "min-nodes", 1, "Minimum autoscaler node count")
	createCmd.Flags().IntVar(&createMaxNodes, "max-nodes", 1, "Maximum autoscaler node count")
	_ = createCmd.MarkFlagRequired("account")
	_ = createCmd.MarkFlagRequired("region")
}

func generateTerraformFiles(terraformRepo string, vars gitops.ClusterVars) error {
	clusterDir := filepath.Join(terraformRepo, "clusters", vars.Name)
	tmplBase := "cluster/" + vars.Provider // e.g. cluster/do or cluster/aws

	files := []struct{ tmpl, dest string }{
		{tmplBase + "/main.tf.tmpl", "main.tf"},
		{tmplBase + "/backend.tf.tmpl", "backend.tf"},
		{tmplBase + "/outputs.tf.tmpl", "outputs.tf"},
		{tmplBase + "/variables.tf.tmpl", "variables.tf"},
	}
	for _, f := range files {
		if err := gitops.RenderFromFS(templates.FS, f.tmpl, filepath.Join(clusterDir, f.dest), vars); err != nil {
			return err
		}
	}
	return nil
}

// updatePrimaryClusterWorkspace rewrites the primary_cluster_workspace line in
// shared/cloudflare/terraform.tfvars so the next apply points apex DNS at the new cluster.
func updatePrimaryClusterWorkspace(cfDir, workspaceName string) error {
	tvarsPath := filepath.Join(cfDir, "terraform.tfvars")
	raw, err := os.ReadFile(tvarsPath)
	if err != nil {
		return fmt.Errorf("read terraform.tfvars: %w", err)
	}
	updated := regexp.MustCompile(`(?m)^primary_cluster_workspace\s*=\s*"[^"]*"`).
		ReplaceAllString(string(raw), fmt.Sprintf(`primary_cluster_workspace = "%s"`, workspaceName))
	if updated == string(raw) {
		updated += fmt.Sprintf("\nprimary_cluster_workspace = %q\n", workspaceName)
	}
	return os.WriteFile(tvarsPath, []byte(updated), 0644)
}

// bootstrapOPServiceAccountSecret creates the single secret ESO needs before
// ArgoCD has had a chance to deploy anything: the 1Password service account token.
// Pre-creating it lets the ClusterSecretStore come up healthy on the first sync.
func bootstrapOPServiceAccountSecret(ctx context.Context, kube *kubectl.Client, token string) error {
	if err := kube.EnsureNamespace(ctx, "external-secrets"); err != nil {
		return fmt.Errorf("ensure namespace: %w", err)
	}
	if err := kube.CreateOpaqueSecret(ctx, "external-secrets", "op-service-account-token", map[string][]byte{
		"token": []byte(token),
	}); err != nil {
		return fmt.Errorf("create op-service-account-token: %w", err)
	}
	return nil
}

func generateDieubernetesFiles(dieubernetesRepo string, vars gitops.ClusterVars) error {
	clusterDir := filepath.Join(dieubernetesRepo, "clusters", vars.Name)
	tmplBase := "cluster/" + vars.Provider

	for _, f := range []struct{ tmpl, dest string }{
		{tmplBase + "/cluster.yaml.tmpl", "cluster.yaml"},
		{tmplBase + "/envoy-gateway.yaml.tmpl", filepath.Join("overrides", "envoy-gateway.yaml")},
		{tmplBase + "/metrics-server.yaml.tmpl", filepath.Join("overrides", "metrics-server.yaml")},
	} {
		if err := gitops.RenderFromFS(templates.FS, f.tmpl, filepath.Join(clusterDir, f.dest), vars); err != nil {
			return err
		}
	}
	return nil
}

