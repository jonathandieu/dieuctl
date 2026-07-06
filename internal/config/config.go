package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	TerraformRepo    string `mapstructure:"terraform_repo"`
	DieubernetesRepo string `mapstructure:"dieubernetes_repo"`

	OnePassword OnePasswordConfig `mapstructure:"onepassword"`
	TFC         TFCConfig         `mapstructure:"tfc"`
	Cloudflare  CloudflareConfig  `mapstructure:"cloudflare"`
	ArgoCD      ArgoCDConfig      `mapstructure:"argocd"`
	Providers   ProvidersConfig   `mapstructure:"providers"`
	Traffic     TrafficConfig     `mapstructure:"traffic"`
}

type OnePasswordConfig struct {
	Vault               string `mapstructure:"vault"`
	ServiceAccountToken string `mapstructure:"service_account_token"` // op:// reference to ESO's 1Password service account token
}

type TFCConfig struct {
	Organization string `mapstructure:"organization"`
	Token        string `mapstructure:"token"` // op:// reference
}

type CloudflareConfig struct {
	APIToken string `mapstructure:"api_token"` // op:// reference
	ZoneID   string `mapstructure:"zone_id"`   // op:// reference
	Domain   string `mapstructure:"domain"`
}

type ArgoCDConfig struct {
	Server          string `mapstructure:"server"`
	Token           string `mapstructure:"token"`           // op:// reference
	AdminPassword   string `mapstructure:"admin_password"`  // op:// reference, resolved + bcrypted at bootstrap
	PlatformCluster string `mapstructure:"platform_cluster"` // cluster name hosting ArgoCD, e.g. dieubernetes-platform-do-atl1
}

type ProvidersConfig struct {
	DigitalOcean DigitalOceanProviderConfig `mapstructure:"digitalocean"`
}

type DigitalOceanProviderConfig struct {
	Accounts map[string]DOAccountConfig `mapstructure:"accounts"`
}

type DOAccountConfig struct {
	Token          string `mapstructure:"token"`           // op:// reference
	TFCVariableSet string `mapstructure:"tfc_variable_set"` // variable set name in TFC
}

type TrafficConfig struct {
	TFCWorkspace    string `mapstructure:"tfc_workspace"`
	PreviousCluster string `mapstructure:"previous_cluster"` // written by traffic switch
}

// Load reads and unmarshals the config from the default or overridden path.
func Load(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		viper.AddConfigPath(filepath.Join(home, ".config", "dieuctl"))
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.TerraformRepo = expandHome(cfg.TerraformRepo)
	cfg.DieubernetesRepo = expandHome(cfg.DieubernetesRepo)

	return &cfg, nil
}

// DOAccount returns the config for a named DigitalOcean account, or an error
// if the account is not defined in config.
func (c *Config) DOAccount(name string) (DOAccountConfig, error) {
	acct, ok := c.Providers.DigitalOcean.Accounts[name]
	if !ok {
		names := make([]string, 0, len(c.Providers.DigitalOcean.Accounts))
		for k := range c.Providers.DigitalOcean.Accounts {
			names = append(names, k)
		}
		sort.Strings(names)
		return DOAccountConfig{}, fmt.Errorf("unknown account %q, configured: %s", name, strings.Join(names, ", "))
	}
	return acct, nil
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
