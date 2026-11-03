package secrets

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Target is one named 1Password item (or alias list) plus the checker that
// validates the credentials inside it.
type Target struct {
	Name string
	Kind string
	// Items are candidate item titles; the first that exists in the vault wins.
	// New-convention titles go first, legacy titles after.
	Items []string
	// Fields restricts which labels are probed for token kinds. Empty probes
	// every non-empty concealed field.
	Fields []string
	Params Params
}

// Kinds returns the checker kinds wired to production endpoints.
func Kinds(c *http.Client) map[string]Kind {
	token := func(f func(Params) Checker) Kind { return Kind{New: f} }
	return map[string]Kind{
		"anthropic": token(func(Params) Checker { return NewAnthropic(c, "https://api.anthropic.com") }),
		"openai":    token(func(Params) Checker { return NewOpenAI(c, "https://api.openai.com") }),
		"digitalocean": token(func(Params) Checker {
			return NewDigitalOcean(c, "https://api.digitalocean.com")
		}),
		"github": token(func(Params) Checker { return NewGitHub(c, "https://api.github.com") }),
		"tfc":    token(func(Params) Checker { return NewTFC(c, "https://app.terraform.io") }),
		"cloudflare": token(func(p Params) Checker {
			return NewCloudflare(c, "https://api.cloudflare.com", p.AccountID)
		}),
		"digitalocean-spaces": {
			Fields: map[string]string{"access_key_id": "access_key_id", "secret_access_key": "secret_access_key"},
			New:    func(p Params) Checker { return NewSpaces(c, p.Region, p.Bucket) },
		},
	}
}

// credentialOnly pins items that hold other concealed fields (zone ids and
// the like) to the single token field.
var credentialOnly = []string{"credential"}

// BuiltinTargets are the credentials dieuctl knows about by default.
// DigitalOcean is split by account, one Spaces pair per account. Anthropic,
// OpenAI and DigitalOcean items probe every non-empty concealed field, since
// one item may hold several keys.
func BuiltinTargets() []Target {
	return []Target{
		{Name: "anthropic.api-key", Kind: "anthropic", Items: []string{"anthropic.api-key", "Anthropic"}},
		{Name: "openai.api-key", Kind: "openai", Items: []string{"openai.api-key", "OpenAI"}},
		{Name: "digitalocean.personal", Kind: "digitalocean", Items: []string{"digitalocean.personal"}},
		{Name: "digitalocean.personal-spaces", Kind: "digitalocean-spaces", Items: []string{"digitalocean.personal-spaces"}},
		{Name: "digitalocean.work", Kind: "digitalocean", Items: []string{"digitalocean.work"}},
		{Name: "digitalocean.work-spaces", Kind: "digitalocean-spaces", Items: []string{"digitalocean.work-spaces"}},
		{Name: "cloudflare.api-token", Kind: "cloudflare", Items: []string{"cloudflare.api-token"}, Fields: credentialOnly},
		{Name: "github.ghcr-pull", Kind: "github", Items: []string{"github.ghcr-pull"}, Fields: credentialOnly},
		{Name: "hcp-terraform.token", Kind: "tfc", Items: []string{"hcp-terraform.token", "HCP Terraform"}, Fields: credentialOnly},
	}
}

// Config is the optional `secrets:` block in dieuctl's config file.
type Config struct {
	SpacesRegion string         `mapstructure:"spaces_region"`
	Targets      []TargetConfig `mapstructure:"targets"`
}

// TargetConfig adds or overrides a target by name. Only non-secret settings
// live here; credentials stay in 1Password.
type TargetConfig struct {
	Name      string   `mapstructure:"name"`
	Checker   string   `mapstructure:"checker"`
	Item      string   `mapstructure:"item"`
	Items     []string `mapstructure:"items"`
	Fields    []string `mapstructure:"fields"`
	Region    string   `mapstructure:"region"`
	Bucket    string   `mapstructure:"bucket"`
	AccountID string   `mapstructure:"account_id"`
}

// BuildTargets merges config over the built-ins: a config target with a
// built-in's name replaces it in place, any other is appended.
func BuildTargets(cfg Config, kinds map[string]Kind) ([]Target, error) {
	region := cfg.SpacesRegion
	if region == "" {
		region = DefaultSpacesRegion
	}
	targets := BuiltinTargets()
	for i := range targets {
		targets[i].Params.Region = region
	}
	index := make(map[string]int, len(targets))
	for i, t := range targets {
		index[t.Name] = i
	}

	for _, tc := range cfg.Targets {
		if tc.Name == "" {
			return nil, fmt.Errorf("secrets.targets: entry missing name")
		}
		if _, ok := kinds[tc.Checker]; !ok {
			return nil, fmt.Errorf("secrets.targets[%s]: unknown checker %q, available: %s", tc.Name, tc.Checker, strings.Join(kindNames(kinds), ", "))
		}
		items := tc.Items
		if tc.Item != "" {
			items = append([]string{tc.Item}, items...)
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("secrets.targets[%s]: item or items required", tc.Name)
		}
		t := Target{
			Name: tc.Name, Kind: tc.Checker, Items: items, Fields: tc.Fields,
			Params: Params{Region: tc.Region, Bucket: tc.Bucket, AccountID: tc.AccountID},
		}
		if t.Params.Region == "" {
			t.Params.Region = region
		}
		if i, ok := index[t.Name]; ok {
			targets[i] = t
		} else {
			index[t.Name] = len(targets)
			targets = append(targets, t)
		}
	}
	return targets, nil
}

// Select returns the named targets in registry order, or all when names is empty.
func Select(targets []Target, names []string) ([]Target, error) {
	if len(names) == 0 {
		return targets, nil
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var out []Target
	for _, t := range targets {
		if want[t.Name] {
			out = append(out, t)
			delete(want, t.Name)
		}
	}
	if len(want) > 0 {
		unknown := make([]string, 0, len(want))
		for n := range want {
			unknown = append(unknown, n)
		}
		sort.Strings(unknown)
		known := make([]string, len(targets))
		for i, t := range targets {
			known[i] = t.Name
		}
		return nil, fmt.Errorf("unknown target %s, available: %s", strings.Join(unknown, ", "), strings.Join(known, ", "))
	}
	return out, nil
}

func kindNames(kinds map[string]Kind) []string {
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
