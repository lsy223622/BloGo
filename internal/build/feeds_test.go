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

func TestBuildValidationRequiresAllFeedFormats(t *testing.T) {
	c := buildFixture(t)
	result, err := Build(c, filepath.Join(t.TempDir(), "site"), false)
	if err != nil {
		t.Fatal(err)
	}
	pages := []*site.Page{}
	for _, page := range result.Manifest.Pages {
		pages = append(pages, &site.Page{ID: page.ID, URL: page.URL})
	}
	for _, file := range []string{"rss.xml", "feed/rss/index.xml", "feed/atom/index.xml"} {
		path := filepath.Join(result.Output, "public", file)
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.Remove(path); e != nil {
			t.Fatal(e)
		}
		_, e = validateOutput(filepath.Join(result.Output, "public"), pages, c.Site.BaseURL, false)
		if restore := os.WriteFile(path, data, 0644); restore != nil {
			t.Fatal(restore)
		}
		if e == nil {
			t.Fatal("incomplete feed release accepted", file)
		}
	}
}
