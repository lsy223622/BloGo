package build

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/lsy223622/BloGo/internal/config"
)

type Snapshot struct {
	Root, Fingerprint string
	Config            *config.Config
}

func (s *Snapshot) Close() error { return os.RemoveAll(s.Root) }

// The executable digest changes when builder code or dependencies change. It is
// cached per process and is part of every input fingerprint, including watchers.
var executableDigest = sync.OnceValues(func() (string, error) {
	file, err := os.Executable()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
})

func Scan(c *config.Config, preview bool) (string, error) {
	files, err := collectInputs(c)
	if err != nil {
		return "", err
	}
	return inputFingerprint(files, preview)
}

func collectInputs(c *config.Config) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, group := range []struct {
		label, root string
		allowed     func(string) bool
	}{
		{"content", c.Paths.Source, func(file string) bool {
			return file == "site.yaml" || (strings.HasPrefix(file, "assets/") && staticAllowed(file)) || (strings.HasPrefix(file, "posts/") && path.Ext(file) == ".md") || (strings.HasPrefix(file, "pages/") && (path.Ext(file) == ".md" || path.Ext(file) == ".html"))
		}},
		{"templates", c.Paths.Templates, func(file string) bool { return path.Ext(file) == ".html" || path.Ext(file) == ".tmpl" }},
		{"static", c.Paths.Static, staticAllowed},
	} {
		tree, err := readTree(group.root, group.allowed)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", group.label, err)
		}
		for name, data := range tree {
			files[group.label+"/"+name] = data
		}
	}
	for _, entry := range []struct{ label, file string }{{"config.yaml", c.FilePath()}} {
		info, err := os.Lstat(entry.file)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: expected ordinary input file", entry.file)
		}
		root, err := os.OpenRoot(filepath.Dir(entry.file))
		if err != nil {
			return nil, err
		}
		data, err := root.ReadFile(filepath.Base(entry.file))
		root.Close()
		if err != nil {
			return nil, err
		}
		files[entry.label] = data
		if entry.label == "config.yaml" && sha256.Sum256(data) != c.FileDigest() {
			return nil, fmt.Errorf("configuration changed since load; reload before scanning")
		}
	}
	return files, nil
}

func inputFingerprint(files map[string][]byte, preview bool) (string, error) {
	version, err := executableDigest()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "builder\x00%s\x00preview\x00%s\x00", version, strconv.FormatBool(preview))
	for _, name := range fileKeys(files) {
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(h, "%s\x00%x\x00", name, sum)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func CreateSnapshot(c *config.Config, parent, expected string, preview bool) (*Snapshot, error) {
	files, err := collectInputs(c)
	if err != nil {
		return nil, err
	}
	actual, err := inputFingerprint(files, preview)
	if err != nil {
		return nil, err
	}
	if actual != expected {
		return nil, fmt.Errorf("input changed before snapshot")
	}
	if err = os.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(parent, "snapshot-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(root)
		}
	}()
	for _, group := range []string{"content", "templates", "static"} {
		if err = os.MkdirAll(filepath.Join(root, group), 0755); err != nil {
			return nil, err
		}
	}
	for _, file := range fileKeys(files) {
		if err = writeFile(root, file, files[file]); err != nil {
			return nil, err
		}
	}
	copyFiles, err := readTree(root, func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	copied, err := inputFingerprint(copyFiles, preview)
	if err != nil {
		return nil, err
	}
	if copied != expected {
		return nil, fmt.Errorf("snapshot fingerprint mismatch")
	}
	cloned := *c
	cloned.Paths.Source = filepath.Join(root, "content")
	cloned.Paths.Templates = filepath.Join(root, "templates")
	cloned.Paths.Static = filepath.Join(root, "static")
	cloned.Paths.Taxonomies = filepath.Join(root, "content/site.yaml")
	cloned.Paths.Assets = filepath.Join(root, "content/assets")
	if err = os.MkdirAll(cloned.Paths.Assets, 0755); err != nil {
		return nil, err
	}
	settings, err := config.LoadContentSettings(cloned.Paths.Taxonomies)
	if err != nil {
		return nil, err
	}
	cloned.ApplyContentSettings(settings)
	success = true
	return &Snapshot{Root: root, Fingerprint: copied, Config: &cloned}, nil
}
