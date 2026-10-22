package site

import (
	"sort"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

type SiteIndex struct {
	ByID        map[string]*content.ContentItem
	ByURL       map[string]*content.ContentItem
	ByPublished []*content.ContentItem
	ByUpdated   []*content.ContentItem
	PublicPosts []*content.ContentItem
	Pages       []*content.ContentItem
	ByCategory  map[string][]*content.ContentItem
	ByTag       map[string][]*content.ContentItem
	ByMonth     map[string][]*content.ContentItem
}

func NewIndex(items []*content.ContentItem, preview bool) (*SiteIndex, error) {
	visible := make([]*content.ContentItem, 0, len(items))
	for _, i := range items {
		if preview || i.ProductionVisible() {
			visible = append(visible, i)
		}
	}
	if err := ValidateRoutes(visible, nil); err != nil {
		return nil, err
	}
	index := &SiteIndex{ByID: map[string]*content.ContentItem{}, ByURL: map[string]*content.ContentItem{}}
	for _, i := range visible {
		index.ByID[i.ID] = i
		index.ByURL[i.URL] = i
		if i.Kind == content.Page {
			index.Pages = append(index.Pages, i)
			continue
		}
		if i.Status == content.Public {
			index.PublicPosts = append(index.PublicPosts, i)
		}
		if i.Status == content.Public || (preview && i.Status == content.Draft) {
			index.ByPublished = append(index.ByPublished, i)
			index.ByUpdated = append(index.ByUpdated, i)
		}
	}
	SortByDate(index.ByPublished, func(i *content.ContentItem) *time.Time { return i.Published })
	SortByDate(index.ByUpdated, func(i *content.ContentItem) *time.Time { return i.Updated })
	SortByDate(index.PublicPosts, func(i *content.ContentItem) *time.Time { return i.Published })
	sort.Slice(index.Pages, func(a, b int) bool { return index.Pages[a].ID < index.Pages[b].ID })
	return index, nil
}

func SortByDate(items []*content.ContentItem, date func(*content.ContentItem) *time.Time) {
	sort.Slice(items, func(a, b int) bool {
		x, y := date(items[a]), date(items[b])
		if x == nil && y != nil {
			return false
		}
		if x != nil && y == nil {
			return true
		}
		if x != nil && y != nil && !x.Equal(*y) {
			return x.After(*y)
		}
		return items[a].ID < items[b].ID
	})
}
