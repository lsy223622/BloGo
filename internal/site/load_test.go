package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

func TestGateA(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	items, err := LoadContent("../../content", "../../taxonomies.yaml", loc, false)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*content.ContentItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if len(items) != 7 || byID["page:about"].Published != nil || byID["page:lab"].PubliclyListed() || byID["page:lab"].Stats || len(byID["post:welcome"].Meta.Categories) != 1 || byID["post:review-copy"].PubliclyListed() || !byID["post:review-copy"].Stats || byID["post:first-steps"] == nil || byID["post:release-notes"] == nil {
		t.Fatalf("%+v", items)
	}
}

func TestTaxonomySelectionBeforeIndexing(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "content")
	if err := os.MkdirAll(filepath.Join(source, "posts"), 0700); err != nil {
		t.Fatal(err)
	}
	taxFile := filepath.Join(root, "taxonomies.yaml")
	taxData := "schema_version: 1\ncategories:\n  tech: {name: Tech}\n  unused: {name: Unused category}\ntags:\n  used: {name: Used}\n  unused: {name: Unused tag}\n"
	if err := os.WriteFile(taxFile, []byte(taxData), 0600); err != nil {
		t.Fatal(err)
	}
	for name, metadata := range map[string]string{
		"keep":          "categories: [missing, tech, tech]\ntags: [missing, used, used]\n",
		"uncategorized": "tags: [missing]\n",
	} {
		body := "---\ntitle: " + name + "\nstatus: public\npublished: 2026-10-23\n" + metadata + "---\nBody"
		if err := os.WriteFile(filepath.Join(source, "posts", name+".md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{"public", "unlisted", "draft"} {
		// Excluded sources must not reserve IDs or URLs of retained articles.
		body := "---\nid: keep\nslug: keep\ntitle: Excluded\nstatus: " + status + "\npublished: 2026-10-23\ncategories: [missing, another-missing]\n---\nBody"
		if err := os.WriteFile(filepath.Join(source, "posts", status+".md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, preview := range []bool{false, true} {
		items, err := LoadContent(source, taxFile, time.UTC, preview)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || items[0].ID != "post:keep" || items[1].ID != "post:uncategorized" {
			t.Fatalf("preview %v: unexpected items %+v", preview, items)
		}
		if strings.Join(items[0].Meta.Tags, ",") != "used" || len(items[1].Meta.Tags) != 0 {
			t.Fatal("unconfigured tags retained")
		}
		if strings.Join(items[0].Meta.Categories, ",") != "tech" {
			t.Fatal("mixed categories were not filtered or deduplicated")
		}
		tax, err := content.ReadTaxonomies(taxFile)
		if err != nil {
			t.Fatal(err)
		}
		index, err := NewIndex(items, preview)
		if err != nil {
			t.Fatal(err)
		}
		pages, err := BuildPages(index, tax, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pages {
			if p.ID == "categories" && (len(p.Terms) != 1 || p.Terms[0].ID != "tech") {
				t.Fatalf("categories: %+v", p.Terms)
			}
			if p.ID == "tags" && (len(p.Terms) != 1 || p.Terms[0].ID != "used") {
				t.Fatalf("tags: %+v", p.Terms)
			}
		}
	}
}

func TestDraftFilteringAndDeterministicPages(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "content")
	if err := os.MkdirAll(filepath.Join(src, "pages"), 0700); err != nil {
		t.Fatal(err)
	}
	tax := filepath.Join(root, "taxonomies.yaml")
	if err := os.WriteFile(tax, []byte("schema_version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, data string }{{"z", "id: a\nstatus: public\ntitle: A\n"}, {"a", "id: z\nstatus: unlisted\ntitle: Z\n"}, {"draft", ""}} {
		if err := os.WriteFile(filepath.Join(src, "pages", f.name+".md"), []byte("---\n"+f.data+"extra: {}\n---\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for n := 0; n < 2; n++ {
		items, err := LoadContent(src, tax, time.UTC, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || items[0].ID != "page:a" || items[1].ID != "page:z" {
			t.Fatalf("items %+v", items)
		}
	}
	items, err := LoadContent(src, tax, time.UTC, true)
	if err != nil || len(items) != 3 {
		t.Fatalf("preview: %v %+v", err, items)
	}
}
