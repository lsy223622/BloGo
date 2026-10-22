package site

import (
	"reflect"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

func indexedItem(id string, kind content.Kind, status content.Status, published, updated *time.Time) *content.ContentItem {
	return &content.ContentItem{ID: string(kind) + ":" + id, URL: "/" + string(kind) + "/" + id + "/", Kind: kind, Status: status, Published: published, Updated: updated, Meta: content.Metadata{Title: id}}
}
func ids(items []*content.ContentItem) []string {
	var result []string
	for _, i := range items {
		result = append(result, i.ID)
	}
	return result
}

func TestIndexSortingAndVisibility(t *testing.T) {
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := old.Add(24 * time.Hour)
	a := indexedItem("a", content.Post, content.Public, &old, &recent)
	b := indexedItem("b", content.Post, content.Public, &old, &old)
	hidden := indexedItem("hidden", content.Post, content.Unlisted, &recent, &recent)
	draft := indexedItem("draft", content.Post, content.Draft, nil, nil)
	pageZ := indexedItem("z", content.Page, content.Public, nil, nil)
	pageA := indexedItem("a", content.Page, content.Public, nil, nil)
	items := []*content.ContentItem{pageZ, b, hidden, draft, pageA, a}
	index, err := NewIndex(items, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(index.ByPublished), []string{"post:a", "post:b"}) || !reflect.DeepEqual(ids(index.Pages), []string{"page:a", "page:z"}) {
		t.Fatalf("order %v %v", ids(index.ByPublished), ids(index.Pages))
	}
	if index.ByID[hidden.ID] != hidden || index.ByURL[hidden.URL] != hidden || index.ByID[draft.ID] != nil || len(index.PublicPosts) != 2 {
		t.Fatal("visibility")
	}
	a.Updated = &old
	b.Updated = &recent
	other, err := NewIndex([]*content.ContentItem{a, pageA, draft, hidden, b, pageZ}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(index.ByPublished), ids(other.ByPublished)) || !reflect.DeepEqual(ids(other.ByUpdated), []string{"post:b", "post:a"}) {
		t.Fatal("published and updated orders coupled")
	}
	preview, err := NewIndex(items, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ByID[draft.ID] == nil || preview.ByPublished[len(preview.ByPublished)-1] != draft {
		t.Fatal("preview draft ordering")
	}
}
