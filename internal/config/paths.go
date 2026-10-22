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

func writableTarget(path string, directory bool) error {
	info, err := os.Stat(path)
	if err == nil {
		if directory {
			if !info.IsDir() {
				return fmt.Errorf("%s: expected output directory", path)
			}
			return writableDirectory(path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s: expected ordinary output file", path)
		}
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		return f.Close()
	}
	if !os.IsNotExist(err) {
		return err
	}
	parent := path
	if !directory {
		parent = filepath.Dir(path)
	}
	parent, err = existingAncestor(parent)
	if err != nil {
		return err
	}
	return writableDirectory(parent)
}

type namedPath struct{ name, path string }

func (c *Config) validatePaths(command, output string) error {
	inputs := []namedPath{{"source", c.Paths.Source}, {"templates", c.Paths.Templates}, {"static", c.Paths.Static}, {"taxonomies", c.Paths.Taxonomies}, {"config", c.file}, {"manifest_path", c.Stats.ManifestPath}}
	outputs := []namedPath{{"output", output}, {"staging", c.Paths.Staging}, {"releases", c.Paths.Releases}, {"current", c.Paths.Current}, {"public_output", c.Stats.PublicOutput}, {"private_output", c.Stats.PrivateOutput}, {"database_path", c.Stats.DatabasePath}, {"visitor_key_path", c.Stats.VisitorKeyPath}}
	if err := pathRelations(inputs, outputs, true); err != nil {
		return err
	}
	active := map[string]bool{"config": true}
	switch command {
	case "build", "dev", "publish", "watch":
		for _, name := range []string{"source", "templates", "static", "taxonomies", "output"} {
			active[name] = true
		}
		if command == "publish" || command == "watch" {
			for _, name := range []string{"staging", "releases", "current"} {
				active[name] = true
			}
		}
	case "stats":
		for _, name := range []string{"manifest_path", "database_path", "visitor_key_path"} {
			active[name] = true
		}
	case "report":
		for _, name := range []string{"manifest_path", "database_path", "public_output", "private_output"} {
			active[name] = true
		}
	}
	currentLink := false
	if active["current"] {
		info, err := os.Lstat(c.Paths.Current)
		if err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("current must be a symbolic link")
			}
			currentLink = true
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// Other components may be inaccessible to this process. Their declared paths
	// participate in both checks, but only this command's paths are inspected.
	for _, list := range [][]namedPath{inputs, outputs} {
		for i := range list {
			if list[i].path != "" && active[list[i].name] {
				p, err := resolved(list[i].path)
				if err != nil {
					return fmt.Errorf("%s: %w", list[i].name, err)
				}
				list[i].path = p
			}
		}
	}
	if currentLink {
		releases, current := "", ""
		for _, p := range outputs {
			if p.name == "releases" {
				releases = p.path
			}
			if p.name == "current" {
				current = p.path
			}
		}
		if current == releases || !contains(releases, current) {
			return fmt.Errorf("current must point strictly inside releases")
		}
	}
	return pathRelations(inputs, outputs, !active["current"] || currentLink)
}

func pathRelations(inputs, outputs []namedPath, allowCurrentLink bool) error {
	for _, out := range outputs {
		if out.path == "" {
			continue
		}
		for _, in := range inputs {
			// The production Manifest is intentionally within a published release.
			if in.name == "manifest_path" && (out.name == "releases" || out.name == "staging" || out.name == "current") {
				continue
			}
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
				if allowCurrentLink && a.name == "releases" && b.name == "current" && a.path != b.path && contains(a.path, b.path) {
					continue
				}
				return fmt.Errorf("%s overlaps %s", a.name, b.name)
			}
		}
	}
	var public []string
	for _, in := range inputs {
		if in.name == "static" && in.path != "" {
			public = append(public, in.path)
		}
	}
	for _, out := range outputs {
		if out.path == "" {
			continue
		}
		switch out.name {
		case "output", "staging", "releases", "current":
			public = append(public, out.path)
		case "public_output":
			public = append(public, filepath.Dir(out.path))
		}
	}
	for _, secret := range outputs {
		if secret.path == "" {
			continue
		}
		switch secret.name {
		case "database_path", "visitor_key_path", "private_output":
			for _, p := range public {
				if contains(p, secret.path) {
					return fmt.Errorf("%s is inside public output", secret.name)
				}
			}
		}
	}
	return nil
}
