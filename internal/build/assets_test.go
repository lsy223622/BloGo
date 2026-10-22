package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
)

func put(t *testing.T, root, name, data string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAssetsPreserveExplicitPathsAndFingerprintManagedResources(t *testing.T) {
	source := t.TempDir()
	put(t, source, "assets/theme.css", "/* managed */ .title { color: red; }\n")
	put(t, source, "assets/theme.js", "const value = 1 + 2;\n")
	put(t, source, "images/a.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	out := t.TempDir()
	assets, err := CopyAssets(source, out, nil)
	if err != nil {
		t.Fatal(err)
	}
	css := assets["theme.css"]
	if !strings.HasPrefix(css, "/assets/theme.") || css == "/assets/theme.css" {
		t.Fatalf("fingerprint %q", css)
	}
	original, err := os.ReadFile(filepath.Join(out, "assets/theme.css"))
	if err != nil || string(original) != "/* managed */ .title { color: red; }\n" {
		t.Fatal("explicit resource path changed")
	}
	minified, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(strings.TrimPrefix(css, "/"))))
	if err != nil || strings.Contains(string(minified), "managed") {
		t.Fatalf("minify %s: %v", minified, err)
	}
	put(t, source, "assets/theme.css", ".title{color:blue}")
	again, err := CopyAssets(source, t.TempDir(), nil)
	if err != nil || again["theme.css"] == css {
		t.Fatal("changed CSS retained fingerprint")
	}
}

func TestStaticCannotOverridePagesOrServices(t *testing.T) {
	for _, name := range []string{"about/index.html", "about", "data/stats.json", "api/events", "page", "categories/tech"} {
		t.Run(name, func(t *testing.T) {
			source := t.TempDir()
			put(t, source, name, "bad")
			p := &site.Page{ID: "page:about", URL: "/about/", Content: &content.ContentItem{ID: "page:about", URL: "/about/", SourcePath: "pages/about.md"}}
			if _, err := CopyAssets(source, t.TempDir(), []*site.Page{p}); err == nil {
				t.Fatal("accepted conflicting static output")
			}
		})
	}
}

func TestMissingCSSResourceFails(t *testing.T) {
	source := t.TempDir()
	put(t, source, "assets/theme.css", `body{background:url('../images/missing.svg')}`)
	if _, err := CopyAssets(source, t.TempDir(), nil); err == nil {
		t.Fatal("missing stylesheet resource accepted")
	}
}
