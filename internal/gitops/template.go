package gitops

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

// ClusterVars holds the template variables used when generating per-cluster files.
type ClusterVars struct {
	Name      string // e.g. dieubernetes-platform-do-atl1
	ShortName string // e.g. platform-do-atl1 (Name with the dieubernetes- prefix stripped, used for dieubernetes-repo paths and the ArgoCD cluster secret name)
	Provider  string // e.g. do
	Account   string // e.g. platform
	Region    string // e.g. nyc3
	NodeSize  string
	MinNodes  int
	MaxNodes  int

	// Populated after TFC apply
	LBID        string
	LBIP        string
	Environment string // stage | prod (empty for platform cluster)
	Tier        string // platform | workload
}

// RenderFromFS renders a template from an embedded FS into a destination file,
// creating parent directories as needed.
func RenderFromFS(fsys fs.FS, tmplPath, destPath string, vars ClusterVars) error {
	src, err := fs.ReadFile(fsys, tmplPath)
	if err != nil {
		return fmt.Errorf("read template %q: %w", tmplPath, err)
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Parse(string(src))
	if err != nil {
		return fmt.Errorf("parse template %q: %w", tmplPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return fmt.Errorf("render template %q: %w", tmplPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(destPath), err)
	}
	return os.WriteFile(destPath, buf.Bytes(), 0o644)
}
