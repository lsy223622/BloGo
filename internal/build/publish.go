package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/filelock"
	"github.com/lsy223622/BloGo/internal/site"
)

func ReadPublished(root string) (*Result, error) {
	target, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	manifest, err := VerifyRelease(target)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(target, "meta/redirects.json"))
	if err != nil {
		return nil, err
	}
	var redirects map[string]string
	if err = json.Unmarshal(data, &redirects); err != nil {
		return nil, err
	}
	return &Result{Output: target, Manifest: manifest, Redirects: redirects}, nil
}

func PruneReleases(c *config.Config, current string) error {
	return pruneReleases(c.Paths.Releases, current, c.Build.KeepReleases)
}

func Publish(c *config.Config) (*Result, error) {
	if err := publicationSupported(); err != nil {
		return nil, err
	}
	if err := c.Validate("publish", ""); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(c.Paths.Current), 0755); err != nil {
		return nil, err
	}
	lock, err := filelock.Acquire(c.Paths.Current + ".lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	fingerprint, err := Scan(c, false)
	if err != nil {
		return nil, err
	}
	return publishLocked(c, fingerprint)
}

func publishLocked(c *config.Config, fingerprint string) (*Result, error) {
	for _, dir := range []string{c.Paths.Staging, c.Paths.Releases} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	candidate, err := os.MkdirTemp(c.Paths.Staging, "candidate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(candidate)
	result, err := buildSite(c, candidate, false, fingerprint)
	if err != nil {
		return nil, err
	}
	manifest, err := VerifyRelease(candidate)
	if err != nil {
		return nil, err
	}
	if manifest.BuildID != result.Manifest.BuildID || manifest.Fingerprint != result.Manifest.Fingerprint {
		return nil, fmt.Errorf("release changed after build")
	}
	release := filepath.Join(c.Paths.Releases, manifest.BuildID)
	if err = os.Rename(candidate, release); err != nil {
		return nil, err
	}
	if err = atomicCurrent(c.Paths.Current, release, manifest.BuildID); err != nil {
		return nil, err
	}
	result.Output = release
	if err = pruneReleases(c.Paths.Releases, release, c.Build.KeepReleases); err != nil {
		result.Warnings = append(result.Warnings, "release cleanup: "+err.Error())
	}
	return result, nil
}

func VerifyRelease(root string) (*site.Manifest, error) {
	m, err := site.ReadManifest(filepath.Join(root, "meta/manifest.json"))
	if err != nil {
		return nil, err
	}
	fingerprint, err := os.ReadFile(filepath.Join(root, "meta/fingerprint"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(fingerprint)) != m.Fingerprint {
		return nil, fmt.Errorf("release fingerprint mismatch")
	}
	files, err := readTree(filepath.Join(root, "public"), func(file string) bool { return path.Ext(file) == ".html" || staticAllowed(file) })
	if err != nil {
		return nil, err
	}
	expected := map[string]bool{}
	for _, p := range m.Pages {
		file := content.OutputPath(p.URL)
		expected[file] = true
		if _, ok := files[file]; !ok {
			return nil, fmt.Errorf("release page missing %s", p.ID)
		}
	}
	for file := range files {
		if path.Ext(file) == ".html" && !expected[file] {
			return nil, fmt.Errorf("release HTML missing Manifest record %s", file)
		}
	}
	return m, nil
}

func pruneReleases(root, current string, keep int) error {
	type release struct {
		path     string
		manifest *site.Manifest
	}
	var releases []release
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "build-") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		m, err := VerifyRelease(dir)
		if err != nil {
			return err
		}
		if m.BuildID != entry.Name() {
			return fmt.Errorf("release directory/identity mismatch")
		}
		releases = append(releases, release{dir, m})
	}
	sort.Slice(releases, func(i, j int) bool {
		a, b := releases[i].manifest, releases[j].manifest
		if a.GeneratedAt.Equal(b.GeneratedAt) {
			return a.BuildID > b.BuildID
		}
		return a.GeneratedAt.After(b.GeneratedAt)
	})
	retained := 1
	for _, r := range releases {
		if r.path == current {
			continue
		}
		if retained < keep {
			retained++
			continue
		}
		if filepath.Dir(r.path) != filepath.Clean(root) {
			return fmt.Errorf("release cleanup escaped root")
		}
		if err = os.RemoveAll(r.path); err != nil {
			return err
		}
	}
	return nil
}
