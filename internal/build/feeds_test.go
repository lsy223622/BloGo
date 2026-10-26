package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
)

func TestFeedOutputsAndReservedRoutes(t *testing.T) {
	for _, path := range []string{"/feed/", "/feed/rss/", "/feed/atom/", "/feed/rss/index.xml", "/feed/atom/index.xml"} {
		item := &content.ContentItem{ID: "page:collision", URL: path, SourcePath: "pages/collision.md"}
		if err := site.ValidateRoutes([]*content.ContentItem{item}, nil); err == nil {
			t.Fatal("content route collision", path)
		}
	}
	for _, file := range []string{"feed", "feed/rss", "feed/atom", "feed/rss/index.xml", "feed/atom/index.xml"} {
		if err := site.ValidateRoutes(nil, []string{file}); err == nil {
			t.Fatal("static feed collision", file)
		}
	}
	c := buildFixture(t)
	c.Build.StrictLinks = true
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2020-01-01\n---\n[RSS](/feed/) [RDF](/feed/rss/) [Atom](/feed/atom/)")
	result, err := Build(c, filepath.Join(t.TempDir(), "site"), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"rss.xml", "feed/rss/index.xml", "feed/atom/index.xml"} {
		data, e := os.ReadFile(filepath.Join(result.Output, "public", file))
		if e != nil || len(data) == 0 {
			t.Fatal("missing feed", file, e)
		}
	}
}
