package content

import (
	"strings"
	"testing"
	"time"
)

func testTaxonomies(t *testing.T) *Taxonomies {
	t.Helper()
	tax, err := ParseTaxonomies("taxonomies.yaml", []byte("schema_version: 1\ncategories:\n  tech: {name: '技术 / Tech 🛠️'}\n  hardware: {name: '硬件 & PCB', parent: tech}\ntags:\n  esp32: {name: 'ESP32 ⚡'}\n"))
	if err != nil {
		t.Fatal(err)
	}
	return tax
}

func TestMetadataDatesAndVisibility(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tax := testTaxonomies(t)
	for _, tc := range []struct {
		name, source, meta string
		status             Status
		layout             string
		published, updated bool
	}{
		{"public post", "posts/a.md", "title: A\nstatus: public\npublished: 2026-10-23\ncategories: [hardware, hardware]\ntags: [esp32, esp32]\n", Public, "article", true, true},
		{"unlisted", "posts/a.md", "title: A\nstatus: unlisted\npublished: 2026-10-23T01:00:00Z\n", Unlisted, "article", true, true},
		{"default draft", "posts/a.md", "extra: {}\n", Draft, "article", false, false},
		{"undated page", "pages/a.md", "title: A\nstatus: public\n", Public, "page", false, false},
		{"updated page", "pages/a.md", "title: A\nstatus: public\nupdated: 2026-10-23\n", Public, "page", false, true},
		{"html", "pages/a.html", "title: A\nstatus: public\n", Public, "standalone", false, false},
		{"html fragment", "pages/a.html", "layout: page\n", Draft, "page", false, false},
		{"page article", "pages/a.md", "layout: article\n", Draft, "article", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := Parse(tc.source, []byte("---\n"+tc.meta+"---\nBody"))
			if err == nil {
				err = item.Validate(loc, tax)
			}
			if err != nil {
				t.Fatal(err)
			}
			if item.Status != tc.status || item.Layout != tc.layout || (item.Published != nil) != tc.published || (item.Updated != nil) != tc.updated || !item.Stats {
				t.Fatalf("%+v", item)
			}
			if item.ProductionVisible() != (tc.status != Draft) || item.PubliclyListed() != (tc.status == Public) || item.NoIndex() != (tc.status == Unlisted) {
				t.Fatalf("visibility %+v", item)
			}
			if tc.name == "public post" {
				if item.Published.Hour() != 0 || item.Published.Format(time.RFC3339) != "2026-10-23T00:00:00+08:00" || !item.Updated.Equal(*item.Published) || len(item.Meta.Categories) != 1 || len(item.Meta.Tags) != 1 {
					t.Fatalf("dates/dedup %+v", item)
				}
			}
		})
	}
}

func TestMetadataFailures(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tax := testTaxonomies(t)
	for _, tc := range []struct{ source, meta, field string }{
		{"posts/a.md", "status: Public\n", "status"}, {"posts/a.md", "status: ''\n", "status"},
		{"posts/a.md", "status: public\ntitle: A\n", "published"}, {"posts/a.md", "status: unlisted\ntitle: A\n", "published"},
		{"pages/a.md", "status: public\ntitle: '  '\n", "title"},
		{"pages/a.md", "published: 2026-02-30\n", "published"},
		{"pages/a.md", "published: ''\n", "published"},
		{"pages/a.md", "published: 2026-10-23\nupdated: 2026-10-22\n", "updated"},
		{"posts/a.md", "layout: page\n", "layout"}, {"pages/a.md", "layout: standalone\n", "layout"}, {"pages/a.html", "layout: article\n", "layout"},
		{"pages/a.md", "stats: 1\n", "stats"}, {"pages/a.md", "extra: false\n", "extra"},
	} {
		t.Run(tc.meta, func(t *testing.T) {
			item, err := Parse(tc.source, []byte("---\n"+tc.meta+"---\n"))
			if err == nil {
				err = item.Validate(loc, tax)
			}
			if err == nil || !strings.Contains(err.Error(), tc.field) || !strings.Contains(err.Error(), "line ") {
				t.Fatalf("want %s location, got %v", tc.field, err)
			}
		})
	}
}

func TestMetadataKeepsOnlyConfiguredTaxonomies(t *testing.T) {
	item, err := Parse("posts/a.md", []byte("---\ncategories: [unknown, tech, tech]\ntags: [unknown, esp32, ESP32, esp32]\n---\nBody"))
	if err != nil {
		t.Fatal(err)
	}
	if err = item.Validate(time.UTC, testTaxonomies(t)); err != nil {
		t.Fatal(err)
	}
	if len(item.Meta.Tags) != 1 || item.Meta.Tags[0] != "esp32" {
		t.Fatalf("configured tags: %v", item.Meta.Tags)
	}
	if len(item.Meta.Categories) != 1 || item.Meta.Categories[0] != "tech" {
		t.Fatalf("configured categories: %v", item.Meta.Categories)
	}
}

func TestTaxonomiesValidation(t *testing.T) {
	tax := testTaxonomies(t)
	if tax.Categories["tech"].Name != "技术 / Tech 🛠️" || tax.Tags["esp32"].Name != "ESP32 ⚡" {
		t.Fatalf("%+v", tax)
	}
	for _, data := range []string{
		"categories: {}\n", "schema_version: 2\n", "schema_version: '1'\n", "schema_version: 1\nextra: []\n",
		"schema_version: 1\ntags:\n  ESP32: {name: Chip}\n",
		"schema_version: 1\ntags:\n  a: {name: 12}\n",
		"schema_version: 1\ntags:\n  a: {name: ' '}\n",
		"schema_version: 1\ntags:\n  a: {name: \"line\\nfeed\"}\n",
		"schema_version: 1\ntags:\n  a: {name: A, parent: b}\n",
		"schema_version: 1\ncategories:\n  a: {name: A, parent: missing}\n",
		"schema_version: 1\ncategories:\n  a: {name: A, parent: a}\n",
		"schema_version: 1\ncategories:\n  a: {name: A, parent: b}\n  b: {name: B, parent: a}\n",
		"schema_version: 1\ncategories:\n  a: {name: A}\n  a: {name: B}\n",
		"schema_version: 1\nextra:\n  nested: {a: 1, a: 2}\n",
	} {
		if _, err := ParseTaxonomies("taxonomies.yaml", []byte(data)); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
}
