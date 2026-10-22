package build

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/render"
	"github.com/lsy223622/BloGo/internal/site"
)

type Result struct {
	Output    string
	Manifest  *site.Manifest
	Warnings  []string
	Redirects map[string]string
}
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }
func invalid(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return err
	}
	return &ValidationError{Err: err}
}

func Build(c *config.Config, output string, preview bool) (*Result, error) {
	command := "build"
	if preview {
		command = "dev"
	}
	if err := c.Validate(command, output); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(filepath.Dir(c.FilePath()), output)
	}
	fingerprint, err := Scan(c, preview)
	if err != nil {
		return nil, err
	}
	return buildSite(c, output, preview, fingerprint)
}

func buildSite(c *config.Config, output string, preview bool, fingerprint string) (*Result, error) {
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, ".blog-build-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	snapshot, err := CreateSnapshot(c, stage, fingerprint, preview)
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()
	model := snapshot.Config
	loc, err := time.LoadLocation(model.Site.Timezone)
	if err != nil {
		return nil, invalid(err)
	}
	tax, err := content.ReadTaxonomies(model.Paths.Taxonomies)
	if err != nil {
		return nil, invalid(err)
	}
	items, err := site.LoadContent(model.Paths.Source, model.Paths.Taxonomies, loc, preview)
	if err != nil {
		return nil, invalid(err)
	}
	index, err := site.NewIndex(items, preview)
	if err != nil {
		return nil, invalid(err)
	}
	pages, err := site.BuildPages(index, tax, model.Build.PageSize)
	if err != nil {
		return nil, invalid(err)
	}
	site.ApplyNavigation(pages, index)
	public := filepath.Join(stage, "public")
	assets, err := CopyAssets(model.Paths.Static, public, pages)
	if err != nil {
		return nil, invalid(err)
	}
	renderer, err := render.New(model, pages, tax, assets)
	if err != nil {
		return nil, invalid(err)
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	manifest := &site.Manifest{SchemaVersion: 1, BuildID: "build-" + hex.EncodeToString(nonce), Fingerprint: fingerprint, GeneratedAt: time.Now().UTC()}
	for _, page := range pages {
		data, err := renderer.Render(page)
		if err != nil {
			return nil, invalid(fmt.Errorf("%s: %w", page.URL, err))
		}
		if err = writeFile(public, content.OutputPath(page.URL), data); err != nil {
			return nil, err
		}
		manifest.Pages = append(manifest.Pages, site.PageRecord{ID: page.ID, Kind: page.Kind, URL: page.URL, Title: page.Title, Visibility: page.Visibility, StatsEnabled: page.StatsEnabled, Published: page.Published})
	}
	if err = manifest.Validate(preview); err != nil {
		return nil, invalid(err)
	}
	feed, err := site.Feed(index, model.Site.BaseURL, model.Site.Title, model.Build.SummaryMaxChars)
	if err != nil {
		return nil, invalid(err)
	}
	sitemap, err := site.Sitemap(pages, model.Site.BaseURL)
	if err != nil {
		return nil, invalid(err)
	}
	redirects, err := site.Redirects(pages)
	if err != nil {
		return nil, invalid(err)
	}
	rules, err := site.RedirectRules(pages)
	if err != nil {
		return nil, invalid(err)
	}
	for _, file := range []struct {
		name string
		data []byte
	}{{"public/rss.xml", feed}, {"public/sitemap.xml", sitemap}, {"public/robots.txt", site.Robots(model.Site.BaseURL)}, {"meta/redirects.conf", redirects}, {"meta/fingerprint", []byte(fingerprint + "\n")}} {
		if err = writeFile(stage, file.name, file.data); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = writeFile(stage, "meta/manifest.json", append(data, '\n')); err != nil {
		return nil, err
	}
	if err = privateMetadata(filepath.Join(stage, "meta")); err != nil {
		return nil, err
	}
	warnings, err := validateOutput(public, pages, model.Site.BaseURL, model.Build.StrictLinks)
	if err != nil {
		return nil, invalid(err)
	}
	if err = snapshot.Close(); err != nil {
		return nil, err
	}
	if err = os.Chmod(stage, 0755); err != nil {
		return nil, err
	}
	if err = replaceOutput(stage, output); err != nil {
		return nil, err
	}
	return &Result{Output: output, Manifest: manifest, Warnings: warnings, Redirects: rules}, nil
}

func privateMetadata(root string) error {
	if err := os.Chmod(root, 0750); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = os.Chmod(filepath.Join(root, entry.Name()), 0640); err != nil {
			return err
		}
	}
	return nil
}

func replaceOutput(stage, output string) error {
	info, err := os.Lstat(output)
	if os.IsNotExist(err) {
		return os.Rename(stage, output)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output must be an ordinary directory")
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		data, err := os.ReadFile(filepath.Join(output, "meta/manifest.json"))
		if err != nil {
			return fmt.Errorf("refusing to replace a nonempty directory without a build Manifest")
		}
		var previous site.Manifest
		if err = json.Unmarshal(data, &previous); err != nil || previous.SchemaVersion != 1 || previous.BuildID == "" {
			return fmt.Errorf("refusing to replace an unrecognized output directory")
		}
	}
	backup, err := os.MkdirTemp(filepath.Dir(output), ".blog-previous-")
	if err != nil {
		return err
	}
	if err = os.Remove(backup); err != nil {
		return err
	}
	if err = os.Rename(output, backup); err != nil {
		return err
	}
	if err = os.Rename(stage, output); err != nil {
		if rollback := os.Rename(backup, output); rollback != nil {
			return fmt.Errorf("replace failed: %v; restore failed: %w", err, rollback)
		}
		return err
	}
	return os.RemoveAll(backup)
}
