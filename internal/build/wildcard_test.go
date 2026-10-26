package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWildcardAliasesSurvivePublishedSnapshot(t *testing.T) {
	c := buildFixture(t)
	c.Build.StrictLinks = true
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2020-01-01\naliases:\n  - /legacy/example/replies-*\n---\n[Older address](/legacy/example/replies-27)")
	result, err := Build(c, filepath.Join(t.TempDir(), "site"), false)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadPublished(result.Output)
	if err != nil || loaded.Redirects["/legacy/example/replies-*"] != "/posts/a/" {
		t.Fatalf("snapshot: %v %v", loaded, err)
	}
	entries, err := os.ReadDir(filepath.Join(result.Output, "public"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "*") || entry.Name() == "legacy" {
			t.Fatal("wildcard emitted as a file")
		}
	}
}
