package site

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

func TestFeedSitemapNavigationAndRedirects(t *testing.T) {
	old := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	recent := old.Add(time.Hour)
	a := indexedItem("a", content.Post, content.Public, &old, &old)
	a.Meta.Title = "A & B"
	a.Body = "正文摘要。"
	a.Aliases = []string{"/old.html"}
	b := indexedItem("b", content.Post, content.Public, &recent, &recent)
	h := indexedItem("hidden", content.Post, content.Unlisted, &recent, &recent)
	i, err := NewIndex([]*content.ContentItem{a, h, b}, false)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := BuildPages(i, &content.Taxonomies{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	ApplyNavigation(pages, i)
	for _, p := range pages {
		if p.ID == a.ID && (p.Next != b || p.Previous != nil) {
			t.Fatal("navigation order")
		}
	}
	feed, err := Feed(i, "https://example.com", "Blog & Site", 160)
	if err != nil {
		t.Fatal(err)
	}
	var rss struct {
		Channel struct {
			Items []struct {
				GUID        string `xml:"guid"`
				Title       string `xml:"title"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err = xml.Unmarshal(feed, &rss); err != nil {
		t.Fatal(err)
	}
	if len(rss.Channel.Items) != 2 || rss.Channel.Items[1].Title != "A & B" || rss.Channel.Items[1].Description != "正文摘要。" || strings.Contains(string(feed), "hidden") {
		t.Fatalf("feed %s", feed)
	}
	sitemap, err := Sitemap(pages, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sitemap), "hidden") || strings.Contains(string(sitemap), "/404/") {
		t.Fatalf("sitemap %s", sitemap)
	}
	redirects, err := Redirects(pages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(redirects), `"~^/old\\.html$" "/post/a/";`) || !strings.Contains(string(redirects), `"~^/post/a/index\\.html$" "/post/a/";`) {
		t.Fatalf("redirects %s", redirects)
	}
	if strings.Contains(string(redirects), "return 200") {
		t.Fatal("invalid redirect response")
	}
}
