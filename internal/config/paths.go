package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolve existing ancestors so links cannot conceal an overlap in an output
// directory that has not been created yet. This validation never creates files.
func resolved(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		_, err = os.Lstat(path)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		path = filepath.Join(path, suffix[i])
	}
	return filepath.Clean(path), nil
}

func contains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (c *Config) validatePaths(output string) error {
	type named struct{ name, path string }
	inputs := []named{{"source", c.Paths.Source}, {"templates", c.Paths.Templates}, {"static", c.Paths.Static}, {"taxonomies", c.Paths.Taxonomies}, {"config", c.file}}
	outputs := []named{{"output", output}, {"staging", c.Paths.Staging}, {"releases", c.Paths.Releases}, {"current", c.Paths.Current}, {"public_output", c.Stats.PublicOutput}, {"private_output", c.Stats.PrivateOutput}, {"database_path", c.Stats.DatabasePath}, {"visitor_key_path", c.Stats.VisitorKeyPath}}
	for _, list := range [][]named{inputs, outputs} {
		for i := range list {
			if list[i].path != "" {
				p, err := resolved(list[i].path)
				if err != nil {
					return fmt.Errorf("%s: %w", list[i].name, err)
				}
				list[i].path = p
			}
		}
	}
	for _, out := range outputs {
		if out.path == "" {
			continue
		}
		for _, in := range inputs {
			if in.path != "" && (contains(in.path, out.path) || contains(out.path, in.path)) {
				return fmt.Errorf("%s overlaps input %s", out.name, in.name)
			}
		}
	}
	for i, a := range outputs {
		if a.path == "" {
			continue
		}
		for _, b := range outputs[i+1:] {
			if b.path == "" {
				continue
			}
			if contains(a.path, b.path) || contains(b.path, a.path) {
				// current intentionally resolves to one of the published releases.
				if (a.name == "releases" && b.name == "current") || (a.name == "current" && b.name == "releases") {
					continue
				}
				return fmt.Errorf("%s overlaps %s", a.name, b.name)
			}
		}
	}
	public := []string{c.Paths.Static, c.Stats.PublicOutput}
	if c.Stats.PublicOutput != "" {
		public[1] = filepath.Dir(c.Stats.PublicOutput)
	}
	for _, root := range []string{output, c.Paths.Staging, c.Paths.Releases, c.Paths.Current} {
		if root != "" {
			public = append(public, root)
		}
	}
	for _, secret := range []named{{"database_path", c.Stats.DatabasePath}, {"visitor_key_path", c.Stats.VisitorKeyPath}, {"private_output", c.Stats.PrivateOutput}} {
		if secret.path == "" {
			continue
		}
		s, err := resolved(secret.path)
		if err != nil {
			return err
		}
		for _, p := range public {
			if p == "" {
				continue
			}
			p, err = resolved(p)
			if err != nil {
				return err
			}
			if contains(p, s) {
				return fmt.Errorf("%s is inside public output", secret.name)
			}
		}
	}
	return nil
}
