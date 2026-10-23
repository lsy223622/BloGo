package site

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

type Term struct {
	ID, Name, Description, URL, Kind, Parent string
	Count                                    int
	Extra                                    map[string]any
}
type Pager struct {
	Number, Total        int
	PreviousURL, NextURL string
}
type Page struct {
	ID, Kind, URL, Title string
	Visibility           content.Status
	StatsEnabled         bool
	Published            *time.Time
	NoIndex              bool
	Content              *content.ContentItem
	Items                []*content.ContentItem
	Terms                []Term
	Pager                Pager
	Previous, Next       *content.ContentItem
}

func BuildPages(index *SiteIndex, tax *content.Taxonomies, pageSize int) ([]*Page, error) {
	if pageSize < 1 || tax == nil {
		return nil, fmt.Errorf("positive page size and taxonomies required")
	}
	index.ByCategory = map[string][]*content.ContentItem{}
	index.ByTag = map[string][]*content.ContentItem{}
	index.ByMonth = map[string][]*content.ContentItem{}
	years := map[string][]*content.ContentItem{}
	for _, item := range index.PublicPosts {
		seen := map[string]bool{}
		for _, id := range item.Meta.Categories {
			for id != "" {
				if seen[id] {
					break
				}
				category, ok := tax.Categories[id]
				if !ok {
					return nil, fmt.Errorf("%s: unknown category %s", item.ID, id)
				}
				seen[id] = true
				index.ByCategory[id] = append(index.ByCategory[id], item)
				id = category.Parent
			}
		}
		for _, id := range item.Meta.Tags {
			if _, ok := tax.Tags[id]; !ok {
				return nil, fmt.Errorf("%s: unknown tag %s", item.ID, id)
			}
			index.ByTag[id] = append(index.ByTag[id], item)
		}
		if item.Published != nil {
			year := item.Published.Format("2006")
			month := item.Published.Format("2006-01")
			years[year] = append(years[year], item)
			index.ByMonth[month] = append(index.ByMonth[month], item)
		}
	}
	var pages []*Page
	ids, urls := map[string]bool{}, map[string]bool{}
	add := func(p *Page) error {
		if ids[p.ID] || urls[p.URL] {
			return fmt.Errorf("duplicate generated page ID or URL: %s %s", p.ID, p.URL)
		}
		ids[p.ID] = true
		urls[p.URL] = true
		pages = append(pages, p)
		return nil
	}
	list := func(id, kind, u, title string, items []*content.ContentItem, terms []Term) error {
		total := (len(items) + pageSize - 1) / pageSize
		if total == 0 {
			total = 1
		}
		for n := 1; n <= total; n++ {
			start := (n - 1) * pageSize
			end := min(start+pageSize, len(items))
			p := &Page{ID: id, Kind: kind, URL: u, Title: title, Visibility: content.Public, StatsEnabled: true, Items: items[start:end], Terms: terms, Pager: Pager{Number: n, Total: total}}
			pageURL := func(number int) string {
				if number == 1 {
					return u
				}
				return u + "page/" + strconv.Itoa(number) + "/"
			}
			if n > 1 {
				p.ID = id + ":page:" + strconv.Itoa(n)
				p.Kind = "pagination"
				p.URL = pageURL(n)
				p.Pager.PreviousURL = pageURL(n - 1)
			}
			if n < total {
				p.Pager.NextURL = pageURL(n + 1)
			}
			if err := add(p); err != nil {
				return err
			}
		}
		return nil
	}
	for _, item := range sortedItems(index.ByID) {
		if err := add(&Page{ID: item.ID, Kind: string(item.Kind), URL: item.URL, Title: item.Meta.Title, Visibility: item.Status, StatsEnabled: item.Stats, Published: item.Published, NoIndex: item.NoIndex(), Content: item}); err != nil {
			return nil, err
		}
	}
	if err := list("home", "system", "/", "首页", index.ByPublished, nil); err != nil {
		return nil, err
	}
	if err := list("updates", "system", "/updates/", "最近更新", index.ByUpdated, nil); err != nil {
		return nil, err
	}
	var tags, archives []Term
	for _, id := range keys(index.ByCategory) {
		c := tax.Categories[id]
		if err := list("category:"+id, "category", "/categories/"+id+"/", c.Name, index.ByCategory[id], nil); err != nil {
			return nil, err
		}
	}
	for _, id := range keys(index.ByTag) {
		tag := tax.Tags[id]
		term := Term{ID: id, Name: tag.Name, Description: tag.Description, URL: "/tags/" + id + "/", Kind: "tag", Count: len(index.ByTag[id]), Extra: tag.Extra}
		tags = append(tags, term)
		if err := list("tag:"+id, "tag", term.URL, tag.Name, index.ByTag[id], nil); err != nil {
			return nil, err
		}
	}
	for _, year := range keys(years) {
		u := "/archives/" + year + "/"
		archives = append(archives, Term{ID: year, Name: year, URL: u, Kind: "archive", Count: len(years[year])})
		if err := list("archive:"+year, "archive", u, year, years[year], nil); err != nil {
			return nil, err
		}
	}
	for _, month := range keys(index.ByMonth) {
		u := "/archives/" + month[:4] + "/" + month[5:] + "/"
		archives = append(archives, Term{ID: month, Name: month, URL: u, Kind: "archive", Count: len(index.ByMonth[month])})
		if err := list("archive:"+month, "archive", u, month, index.ByMonth[month], nil); err != nil {
			return nil, err
		}
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].ID > archives[j].ID })
	for _, overview := range []struct {
		id, u, title string
		terms        []Term
	}{{"tags", "/tags/", "标签", tags}, {"archives", "/archives/", "归档", archives}} {
		if err := add(&Page{ID: overview.id, Kind: "system", URL: overview.u, Title: overview.title, Visibility: content.Public, StatsEnabled: true, Terms: overview.terms}); err != nil {
			return nil, err
		}
	}
	if err := add(&Page{ID: "not-found", Kind: "system", URL: "/404/", Title: "页面未找到", Visibility: content.Public, StatsEnabled: true, NoIndex: true}); err != nil {
		return nil, err
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].URL < pages[j].URL })
	return pages, nil
}

func keys[V any](m map[string]V) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
func sortedItems(m map[string]*content.ContentItem) []*content.ContentItem {
	var result []*content.ContentItem
	for _, id := range keys(m) {
		result = append(result, m[id])
	}
	return result
}
