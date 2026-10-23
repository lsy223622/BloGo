package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsy223622/BloGo/internal/site"
)

func TestBuildEndToEndAndReplaceWithoutStaleFiles(t *testing.T) {
	c := buildFixture(t)
	put(t, c.Paths.Source, "pages/about.md", "---\ntitle: About\nstatus: public\n---\nUndated.")
	put(t, c.Paths.Source, "posts/hidden.md", "---\ntitle: Hidden\nstatus: unlisted\npublished: 2026-10-22\n---\nDirect only.")
	put(t, c.Paths.Source, "posts/draft.md", "---\ntitle: Draft\n---\nPreview only.")
	out := filepath.Join(t.TempDir(), "site")
	result, err := Build(c, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Manifest.Pages) != 11 {
		t.Fatalf("page count %d", len(result.Manifest.Pages))
	}
	for _, file := range []string{"public/index.html", "public/posts/a/index.html", "public/404/index.html", "public/about/index.html", "public/rss.xml", "public/sitemap.xml", "public/robots.txt", "meta/manifest.json", "meta/fingerprint"} {
		if _, err = os.Stat(filepath.Join(out, filepath.FromSlash(file))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = os.Stat(filepath.Join(out, "public/posts/draft/index.html")); !os.IsNotExist(err) {
		t.Fatal("draft leaked")
	}
	var manifest site.Manifest
	data, err := os.ReadFile(filepath.Join(out, "meta/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil || manifest.BuildID == "" || manifest.Fingerprint != result.Manifest.Fingerprint {
		t.Fatal("manifest mismatch")
	}
	if err = manifest.Validate(false); err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(out, "public/rss.xml"))
	if err != nil || strings.Contains(string(feed), "Hidden") || strings.Contains(string(feed), "Draft") {
		t.Fatal("feed leak")
	}
	if err = os.Remove(filepath.Join(c.Paths.Source, "pages/about.md")); err != nil {
		t.Fatal(err)
	}
	again, err := Build(c, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Manifest.Pages) != 10 {
		t.Fatal("removed content retained")
	}
	if _, err = os.Stat(filepath.Join(out, "public/about")); !os.IsNotExist(err) {
		t.Fatal("stale output retained")
	}
	preview, err := Build(c, filepath.Join(t.TempDir(), "preview"), true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range preview.Manifest.Pages {
		if p.ID == "post:draft" {
			found = true
		}
	}
	if !found {
		t.Fatal("preview lacks draft")
	}
}

func TestFailedBuildPreservesPreviousOutput(t *testing.T) {
	c := buildFixture(t)
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, out, false); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(out, "public/posts/a/index.html")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: Invalid\nstatus: public\n---\nBody")
	if _, err = Build(c, out, false); err == nil {
		t.Fatal("invalid content built")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(before) != string(after) {
		t.Fatal("failure modified previous output")
	}
}

func TestCategoryConfigurationRemovesAndRestoresAllArticleOutput(t *testing.T) {
	for _, preview := range []bool{false, true} {
		c := buildFixture(t)
		put(t, c.Paths.Source, "posts/a.md", "---\ntitle: Selected article\nstatus: public\npublished: 2026-10-23\ncategories: [missing, tech]\ntags: [known, unknown]\naliases: [/old-a/]\n---\nBody")
		out := filepath.Join(t.TempDir(), "site")
		for _, configured := range []bool{true, false, true} {
			tax := "schema_version: 1\ntags:\n  known: {name: Known}\n"
			if configured {
				tax += "categories:\n  tech: {name: Tech}\n"
			}
			put(t, filepath.Dir(c.Paths.Taxonomies), filepath.Base(c.Paths.Taxonomies), tax)
			result, err := Build(c, out, preview)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, p := range result.Manifest.Pages {
				found = found || p.ID == "post:a"
			}
			if found != configured || (result.Redirects["/old-a/"] != "") != configured {
				t.Fatal("excluded article retained in manifest or redirects")
			}
			for _, file := range []string{"posts/a/index.html", "categories/tech/index.html", "tags/known/index.html"} {
				_, err := os.Stat(filepath.Join(out, "public", filepath.FromSlash(file)))
				if configured && err != nil || !configured && !os.IsNotExist(err) {
					t.Fatalf("configured %v: %s: %v", configured, file, err)
				}
			}
			for _, file := range []string{"index.html", "updates/index.html", "rss.xml", "sitemap.xml"} {
				data, err := os.ReadFile(filepath.Join(out, "public", filepath.FromSlash(file)))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "/posts/a/") != configured {
					t.Fatalf("configured %v: article reference in %s", configured, file)
				}
			}
		}
	}
}

func TestResourceErrorsAndStrictContentLinks(t *testing.T) {
	c := buildFixture(t)
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\n[Missing](/missing/)\n")
	result, err := Build(c, filepath.Join(t.TempDir(), "out"), false)
	if err != nil || len(result.Warnings) != 1 {
		t.Fatalf("warnings %+v, err %v", result, err)
	}
	c.Build.StrictLinks = true
	if _, err = Build(c, filepath.Join(t.TempDir(), "strict"), false); err == nil {
		t.Fatal("strict missing content link built")
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\n![Missing](/images/missing.svg)\n")
	c.Build.StrictLinks = false
	if _, err = Build(c, filepath.Join(t.TempDir(), "asset"), false); err == nil {
		t.Fatal("missing image built")
	}
}

func TestTrackingCoversManifestAndStandaloneRequiresBody(t *testing.T) {
	c := buildFixture(t)
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	c.Paths.Templates = filepath.Join(repo, "templates")
	c.Paths.Static = filepath.Join(repo, "static")
	put(t, c.Paths.Source, "pages/standalone.html", "---\ntitle: Standalone\nstatus: public\nlayout: standalone\n---\n<html><head></head><body><!-- </body> --><script>const s='</body>';</script>Body</body></html>")
	out := filepath.Join(t.TempDir(), "site")
	result, err := Build(c, out, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range result.Manifest.Pages {
		file := filepath.Join(out, "public", filepath.FromSlash(strings.TrimPrefix(p.URL, "/")), "index.html")
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), `data-page-id="`+p.ID+`"`) < 1 || !strings.Contains(string(data), "/assets/stats.") {
			t.Fatalf("missing tracker for %s", p.ID)
		}
	}
	put(t, c.Paths.Source, "pages/standalone.html", "---\ntitle: Broken\nstatus: public\nlayout: standalone\n---\n<html><body>no closing body")
	if _, err = Build(c, out, false); err == nil {
		t.Fatal("bodyless tracked standalone built")
	}
	put(t, c.Paths.Source, "pages/standalone.html", "---\ntitle: Untracked\nstatus: public\nlayout: standalone\nstats: false\n---\n<html><body>original")
	if _, err = Build(c, out, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "public/standalone/index.html"))
	if err != nil || string(data) != "<html><body>original" {
		t.Fatalf("untracked standalone changed: %q %v", data, err)
	}
}

func TestMissingCollectorPreservesPreviousBuild(t *testing.T) {
	c := buildFixture(t)
	out := filepath.Join(t.TempDir(), "site")
	first, err := Build(c, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(c.Paths.Static, "assets/stats.js")); err != nil {
		t.Fatal(err)
	}
	if _, err = Build(c, out, false); err == nil {
		t.Fatal("missing mandatory collector built successfully")
	}
	old, err := site.ReadManifest(filepath.Join(out, "meta/manifest.json"))
	if err != nil || old.BuildID != first.Manifest.BuildID {
		t.Fatal("missing collector replaced old build")
	}
}
