// Package emoji manages a portable archive of Slack custom emoji: images plus
// a manifest recording where each one came from. See dieuctl#24.
package emoji

import (
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
)

const manifestFilename = "manifest.yaml"

// Entry is one emoji's record in the archive manifest.
type Entry struct {
	Filename        string    `yaml:"filename,omitempty"`
	AddedAt         time.Time `yaml:"added_at"`
	SourceWorkspace string    `yaml:"source_workspace,omitempty"`
	// AliasOf is set instead of Filename when the emoji is an alias with no
	// image of its own.
	AliasOf string `yaml:"alias_of,omitempty"`
}

// Manifest is the whole archive's emoji name -> Entry map.
type Manifest map[string]Entry

// LoadManifest reads manifest.yaml from repoDir. A missing file is not an
// error, it returns an empty Manifest so a fresh archive can be built up.
func LoadManifest(repoDir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(repoDir, manifestFilename))
	if os.IsNotExist(err) {
		return Manifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = Manifest{}
	}
	return m, nil
}

// Save writes the manifest back to repoDir.
func (m Manifest) Save(repoDir string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repoDir, manifestFilename), data, 0o644)
}

// EmojisDir is where emoji image files live within the archive repo.
func EmojisDir(repoDir string) string {
	return filepath.Join(repoDir, "emojis")
}
