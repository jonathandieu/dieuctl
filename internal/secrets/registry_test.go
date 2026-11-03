package secrets

import (
	"net/http"
	"strings"
	"testing"
)

func names(ts []Target) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

func TestBuiltinsSplitDigitalOceanByAccount(t *testing.T) {
	got := strings.Join(names(BuiltinTargets()), " ")
	for _, want := range []string{
		"digitalocean.personal", "digitalocean.personal-spaces",
		"digitalocean.work", "digitalocean.work-spaces",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	for _, bad := range []string{"loki", "mimir"} {
		if strings.Contains(got, bad) {
			t.Fatalf("per-bucket target %s must not exist", bad)
		}
	}
}

func TestBuiltinKindsAllRegistered(t *testing.T) {
	kinds := Kinds(http.DefaultClient)
	for _, tg := range BuiltinTargets() {
		if _, ok := kinds[tg.Kind]; !ok {
			t.Fatalf("%s uses unregistered kind %s", tg.Name, tg.Kind)
		}
	}
}

func TestBuildTargetsOverridesAndExtends(t *testing.T) {
	kinds := Kinds(http.DefaultClient)
	ts, err := BuildTargets(Config{
		SpacesRegion: "ams3",
		Targets: []TargetConfig{
			{Name: "digitalocean.work", Checker: "digitalocean", Item: "DigitalOcean PAT - free", Items: []string{"alt"}},
			{Name: "digitalocean.work-spaces", Checker: "digitalocean-spaces", Item: "w", Region: "sfo3", Bucket: "b"},
			{Name: "github.pat", Checker: "github", Item: "github.pat"},
		},
	}, kinds)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Target{}
	for _, tg := range ts {
		by[tg.Name] = tg
	}
	if got := by["digitalocean.work"].Items; len(got) != 2 || got[0] != "DigitalOcean PAT - free" {
		t.Fatalf("override items = %v", got)
	}
	if by["digitalocean.personal-spaces"].Params.Region != "ams3" {
		t.Fatal("global spaces_region not applied")
	}
	if p := by["digitalocean.work-spaces"].Params; p.Region != "sfo3" || p.Bucket != "b" {
		t.Fatalf("params = %+v", p)
	}
	if names(ts)[len(ts)-1] != "github.pat" || len(ts) != len(BuiltinTargets())+1 {
		t.Fatalf("targets = %v", names(ts))
	}
}

func TestBuildTargetsDefaultsToNYC3(t *testing.T) {
	ts, err := BuildTargets(Config{}, Kinds(http.DefaultClient))
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range ts {
		if tg.Kind == "digitalocean-spaces" && tg.Params.Region != "nyc3" {
			t.Fatalf("%s region = %s", tg.Name, tg.Params.Region)
		}
	}
}

func TestBuildTargetsValidation(t *testing.T) {
	kinds := Kinds(http.DefaultClient)
	for _, tc := range []TargetConfig{
		{Checker: "github", Item: "x"},
		{Name: "a", Checker: "nope", Item: "x"},
		{Name: "a", Checker: "github"},
	} {
		if _, err := BuildTargets(Config{Targets: []TargetConfig{tc}}, kinds); err == nil {
			t.Fatalf("expected error for %+v", tc)
		}
	}
}

func TestSelect(t *testing.T) {
	all := BuiltinTargets()
	got, err := Select(all, []string{"openai.api-key", "anthropic.api-key"})
	if err != nil || len(got) != 2 || got[0].Name != "anthropic.api-key" {
		t.Fatalf("got %v err %v", names(got), err)
	}
	if _, err := Select(all, []string{"nope"}); err == nil || !strings.Contains(err.Error(), "available:") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := Select(all, nil); len(got) != len(all) {
		t.Fatal("empty selection must return all")
	}
}
