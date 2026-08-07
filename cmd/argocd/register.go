package argocd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var registerCmd = &cobra.Command{
	Use:   "register <cluster-name>",
	Short: "Register a cluster with ArgoCD on the platform cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if cfg.ArgoCD.PlatformCluster == "" {
			return fmt.Errorf("argocd.platform_cluster is not set in config")
		}
		if cfg.ArgoCD.Server == "" {
			return fmt.Errorf("argocd.server is not set in config")
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}

		adminPassword, err := resolver.Resolve(ctx, cfg.ArgoCD.AdminPassword)
		if err != nil {
			return fmt.Errorf("resolve argocd admin password: %w", err)
		}

		// Read labels from clusters/<name>/cluster.yaml in the dieubernetes repo.
		labels, err := readClusterLabels(cfg.DieubernetesRepo, name)
		if err != nil {
			return fmt.Errorf("read cluster.yaml: %w", err)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		clusterKubeconfig := filepath.Join(home, ".kube", name+".yaml")
		targetContext, err := kubeconfigContext(clusterKubeconfig)
		if err != nil {
			return fmt.Errorf("read context from %s: %w", clusterKubeconfig, err)
		}

		if err := argocdLogin(ctx, cfg.ArgoCD.Server, adminPassword); err != nil {
			return fmt.Errorf("argocd login: %w", err)
		}

		if err := argocdClusterAdd(ctx, targetContext, clusterKubeconfig, name, labels); err != nil {
			return fmt.Errorf("argocd cluster add: %w", err)
		}

		fmt.Printf("registered %q with ArgoCD on %s\n", name, cfg.ArgoCD.PlatformCluster)
		return nil
	},
}

// argocdLogin authenticates the local argocd CLI session against the
// platform cluster's ArgoCD server. argocd cluster add needs an active
// session to register the resulting secret through the API.
func argocdLogin(ctx context.Context, server, password string) error {
	cmd := exec.CommandContext(ctx, "argocd", "login", server,
		"--username", "admin",
		"--password", password,
		"--grpc-web",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

// argocdClusterAdd registers targetContext with ArgoCD via the official CLI.
// This creates a dedicated, least-privilege ServiceAccount and ClusterRole in
// the target cluster instead of reusing an admin credential (see terraform
// ADR 0002 for why the previous approach used an admin bearer token, and why
// this is the preferred fix).
func argocdClusterAdd(ctx context.Context, targetContext, kubeconfigPath, name string, labels map[string]string) error {
	args := []string{
		"cluster", "add", targetContext,
		"--kubeconfig", kubeconfigPath,
		"--name", name,
		"-y",
	}
	for k, v := range labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	cmd := exec.CommandContext(ctx, "argocd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

type clusterYAML struct {
	Labels map[string]string `yaml:"labels"`
}

func readClusterLabels(dieubernetesRepo, clusterName string) (map[string]string, error) {
	path := filepath.Join(dieubernetesRepo, "clusters", clusterName, "cluster.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseClusterLabels(data)
}

func parseClusterLabels(data []byte) (map[string]string, error) {
	var c clusterYAML
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return c.Labels, nil
}

// kubeconfigContext reads the single current-context out of a per-cluster
// kubeconfig file saved by internal/do.SaveKubeconfig. Reading it back
// rather than deriving it from the cluster name avoids coupling to
// DigitalOcean's own context-naming convention.
func kubeconfigContext(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return parseCurrentContext(data)
}

func parseCurrentContext(data []byte) (string, error) {
	var kc struct {
		CurrentContext string `yaml:"current-context"`
	}
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return "", err
	}
	if kc.CurrentContext == "" {
		return "", fmt.Errorf("no current-context set")
	}
	return kc.CurrentContext, nil
}
