package emoji

import (
	"testing"
	"time"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest on missing file: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty manifest for a fresh archive, got %d entries", len(m))
	}

	m["party-parrot"] = Entry{
		Filename:        "party-parrot.gif",
		AddedAt:         time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		SourceWorkspace: "MyTeam",
	}
	m["party-parrot-2"] = Entry{
		AliasOf: "party-parrot",
	}

	if err := m.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest after save: %v", err)
	}

	got, ok := reloaded["party-parrot"]
	if !ok {
		t.Fatal("party-parrot missing after reload")
	}
	if got.Filename != "party-parrot.gif" || got.SourceWorkspace != "MyTeam" || got.AliasOf != "" {
		t.Errorf("party-parrot entry = %+v, unexpected", got)
	}

	alias, ok := reloaded["party-parrot-2"]
	if !ok {
		t.Fatal("party-parrot-2 missing after reload")
	}
	if alias.AliasOf != "party-parrot" || alias.Filename != "" {
		t.Errorf("party-parrot-2 entry = %+v, expected alias with no filename", alias)
	}
}
