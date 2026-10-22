package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGateA(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	items, err := LoadContent("../../content", "../../taxonomies.yaml", loc, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 || items[0].ID != "page:about" || items[0].Published != nil || items[1].ID != "page:lab" || items[1].PubliclyListed() || items[1].Stats || items[3].ID != "post:welcome" || len(items[3].Meta.Categories) != 1 {
		t.Fatalf("%+v", items)
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
