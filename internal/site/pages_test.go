package site

import (
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

func TestAggregatesPaginationAndPageIdentities(t *testing.T) {
	date := time.Date(2026, 10, 23, 1, 0, 0, 0, time.FixedZone("site", 8*3600))
	a := indexedItem("a", content.Post, content.Public, &date, &date)
	a.Meta.Categories = []string{"hardware", "tech"}
	a.Meta.Tags = []string{"pcb"}
	b := indexedItem("b", content.Post, content.Public, &date, &date)
	b.Meta.Categories = []string{"hardware"}
	hidden := indexedItem("hidden", content.Post, content.Unlisted, &date, &date)
	hidden.Meta.Tags = []string{"secret"}
	index, err := NewIndex([]*content.ContentItem{hidden, b, a}, false)
	if err != nil {
		t.Fatal(err)
	}
	tax := &content.Taxonomies{Categories: map[string]content.Category{"tech": {Name: "技术 🛠️"}, "hardware": {Name: "硬件", Parent: "tech"}}, Tags: map[string]content.Tag{"pcb": {Name: "PCB ⚡"}, "secret": {Name: "Secret"}}}
	pages, err := BuildPages(index, tax, 1)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*Page{}
	byURL := map[string]bool{}
	for _, p := range pages {
		if byID[p.ID] != nil || byURL[p.URL] {
			t.Fatal("duplicate page")
		}
		byID[p.ID] = p
		byURL[p.URL] = true
	}
	for _, id := range []string{"home", "home:page:2", "categories", "category:tech", "category:hardware:page:2", "tags", "tag:pcb", "archives", "archive:2026", "archive:2026-10", "updates", "not-found", "post:hidden"} {
		if byID[id] == nil {
			t.Errorf("missing %s", id)
		}
	}
	if byID["tag:secret"] != nil || len(index.ByCategory["tech"]) != 2 || len(byID["categories"].Terms) != 2 || len(byID["tags"].Terms) != 1 {
		t.Fatal("aggregation leak or ancestor duplicate")
	}
	if byID["home"].Pager.NextURL != "/page/2/" || byID["home:page:2"].Pager.PreviousURL != "/" || byID["category:tech"].URL != "/categories/tech/" {
		t.Fatal("pagination/URL")
	}
	tax.Categories["tech"] = content.Category{Name: "Renamed"}
	again, err := BuildPages(index, tax, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range again {
		if byID[p.ID] == nil || byID[p.ID].URL != p.URL {
			t.Fatal("display name changed identity")
		}
	}
}

func TestEmptyAndInvalidPagination(t *testing.T) {
	index, err := NewIndex(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := BuildPages(index, &content.Taxonomies{}, 10)
	if err != nil || len(pages) != 6 {
		t.Fatalf("empty pages %d: %v", len(pages), err)
	}
	if _, err = BuildPages(index, &content.Taxonomies{}, 0); err == nil {
		t.Fatal("accepted zero page size")
	}
}
