package build

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
	"golang.org/x/net/html"
)

var managedHash = regexp.MustCompile(`^assets/(theme|highlight|stats)\.([0-9a-f]{16})\.(css|js)$`)

// Carry only hashes directly referenced by previous HTML. Re-copying inherited
// files would accumulate every historical hash despite release pruning.
func retainPublishedAssets(c *config.Config, candidate string) error {
	type previous struct {
		path     string
		manifest *site.Manifest
	}
	entries, err := os.ReadDir(c.Paths.Releases)
	if err != nil {
		return err
	}
	current, err := filepath.EvalSymlinks(c.Paths.Current)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var releases []previous
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "build-") {
			continue
		}
		dir := filepath.Join(c.Paths.Releases, entry.Name())
		m, err := site.ReadManifest(filepath.Join(dir, "meta/manifest.json"))
		if err != nil {
			return err
		}
		fingerprint, err := os.ReadFile(filepath.Join(dir, "meta/fingerprint"))
		if err != nil {
			return err
		}
		if m.BuildID != entry.Name() || strings.TrimSpace(string(fingerprint)) != m.Fingerprint {
			return fmt.Errorf("invalid previous release %s", entry.Name())
		}
		releases = append(releases, previous{dir, m})
	}
	sort.Slice(releases, func(i, j int) bool {
		a, b := releases[i], releases[j]
		if (a.path == current) != (b.path == current) {
			return a.path == current
		}
		if a.manifest.GeneratedAt.Equal(b.manifest.GeneratedAt) {
			return a.manifest.BuildID > b.manifest.BuildID
		}
		return a.manifest.GeneratedAt.After(b.manifest.GeneratedAt)
	})
	for _, release := range releases[:min(len(releases), c.Build.KeepReleases)] {
		root, err := os.OpenRoot(filepath.Join(release.path, "public"))
		if err != nil {
			return err
		}
		err = copyReferencedHashes(root, release.manifest, filepath.Join(candidate, "public"))
		closeErr := root.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyReferencedHashes(root *os.Root, manifest *site.Manifest, target string) error {
	refs := map[string]bool{}
	for _, page := range manifest.Pages {
		data, err := root.ReadFile(content.OutputPath(page.URL))
		if err != nil {
			return err
		}
		z := html.NewTokenizer(bytes.NewReader(data))
		for {
			kind := z.Next()
			if kind == html.ErrorToken {
				break
			}
			if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
				continue
			}
			token := z.Token()
			if token.Data != "link" && token.Data != "script" {
				continue
			}
			for _, a := range token.Attr {
				if a.Key != "src" && a.Key != "href" {
					continue
				}
				file := strings.TrimPrefix(a.Val, "/")
				if managedHash.MatchString(file) {
					refs[file] = true
				}
			}
		}
	}
	for file := range refs {
		info, err := root.Lstat(file)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("previous asset is not an ordinary file: %s", file)
		}
		data, err := root.ReadFile(file)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if fmt.Sprintf("%x", sum[:8]) != managedHash.FindStringSubmatch(file)[2] {
			return fmt.Errorf("previous asset hash mismatch: %s", file)
		}
		existing, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(file)))
		if err == nil {
			if !bytes.Equal(existing, data) {
				return fmt.Errorf("managed asset hash collision: %s", file)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err = writeFile(target, file, data); err != nil {
			return err
		}
	}
	return nil
}
