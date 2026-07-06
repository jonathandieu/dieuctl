package config

import (
	"os"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"tilde path expands", "~/Workspace/terraform", home + "/Workspace/terraform"},
		{"absolute path unchanged", "/etc/dieuctl/config.yaml", "/etc/dieuctl/config.yaml"},
		{"relative path unchanged", "config.yaml", "config.yaml"},
		{"empty string unchanged", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandHome(tt.path); got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestConfig_DOAccount(t *testing.T) {
	cfg := &Config{
		Providers: ProvidersConfig{
			DigitalOcean: DigitalOceanProviderConfig{
				Accounts: map[string]DOAccountConfig{
					"platform": {Token: "op://vault/platform/token"},
				},
			},
		},
	}

	got, err := cfg.DOAccount("platform")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Token != "op://vault/platform/token" {
		t.Errorf("Token = %q, want %q", got.Token, "op://vault/platform/token")
	}
}

func TestConfig_DOAccount_Unknown(t *testing.T) {
	cfg := &Config{
		Providers: ProvidersConfig{
			DigitalOcean: DigitalOceanProviderConfig{
				Accounts: map[string]DOAccountConfig{
					"spare":    {},
					"platform": {},
					"main":     {},
				},
			},
		},
	}

	_, err := cfg.DOAccount("nonexistent")
	if err == nil {
		t.Fatal("expected an error for an unknown account, got nil")
	}
	want := `unknown account "nonexistent", configured: main, platform, spare`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
