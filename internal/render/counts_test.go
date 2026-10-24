package render

import (
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
	"strings"
	"testing"
	"time"
)

func TestListViewCountNodesAndBrand(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c.Content.Logo = "/media/brand/logo.svg"
	c.Content.Favicon = "/media/brand/favicon.svg"
	tax := c.Content.Taxonomies()
	zone, _ := time.LoadLocation(c.Site.Timezone)
	items, err := site.LoadContentWithTaxonomies(c.Paths.Source, tax, zone, true)
	if err != nil {
		t.Fatal(err)
	}
	index, err := site.NewIndex(items, true)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := site.BuildPages(index, tax, 1)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := New(c, pages, tax, map[string]string{"stats.js": "/assets/stats.abc.js"})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, p := range pages {
		if p.Content != nil && p.Content.Layout == "standalone" {
			continue
		}
		output, e := renderer.Render(p)
		if e != nil {
			t.Fatal(e)
		}
		html := string(output)
		if !strings.Contains(html, `href="/media/brand/favicon.svg"`) || !strings.Contains(html, "/media/brand/logo.svg") {
			t.Fatal("configured brand absent")
		}
		for _, item := range p.Items {
			if item.Stats && item.Status == content.Public && !strings.Contains(html, `data-views-for="`+item.ID+`"`) {
				t.Fatalf("missing %s count on %s", item.ID, p.ID)
			}
		}
		if len(p.Items) > 0 {
			kinds[p.Kind] = true
		}
		if p.Visibility == content.Draft && strings.Contains(html, `data-stats-src="/assets/stats.abc.js"`) {
			t.Fatal("draft collects statistics")
		}
	}
	for _, kind := range []string{"system", "pagination", "category", "tag", "archive"} {
		if !kinds[kind] {
			t.Fatal("untested list kind", kind)
		}
	}
	c.Stats.Enabled = false
	output, err := renderer.Render(&site.Page{ID: "home", URL: "/", StatsEnabled: true, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), `src="/assets/stats.abc.js"`) || strings.Contains(string(output), `data-views-for=`) {
		t.Fatal("disabled statistics markup")
	}
}
