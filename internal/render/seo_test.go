package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
	"golang.org/x/net/html"
)

func headValues(t *testing.T, data []byte) (map[string]string, []string) {
	t.Helper()
	values := map[string]string{}
	var schemas []string
	z := html.NewTokenizer(bytes.NewReader(data))
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				t.Fatal(z.Err())
			}
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		attrs := map[string]string{}
		for _, attr := range token.Attr {
			attrs[attr.Key] = attr.Val
		}
		switch token.Data {
		case "title":
			z.Next()
			values["title"] = string(z.Text())
		case "meta":
			key := attrs["name"]
			if attrs["property"] != "" {
				key = attrs["property"]
			}
			values[key] = attrs["content"]
		case "link":
			if attrs["rel"] == "canonical" {
				values["canonical"] = attrs["href"]
			}
		case "script":
			if attrs["type"] == "application/ld+json" {
				z.Next()
				schemas = append(schemas, string(z.Text()))
			}
		}
	}
	return values, schemas
}

func TestGeneratedSEOAndPaginationMetadata(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c.Site.Title = "站名 & Site"
	c.Site.Subtitle = "真实简介 <生活>"
	c.Site.BaseURL = "https://example.test"
	tax := &content.Taxonomies{Categories: map[string]content.Category{"tech": {Name: "技术", Description: "技术介绍"}}, Tags: map[string]content.Tag{"friends": {Name: "友链", Description: "标签介绍"}}}
	renderer, err := New(c, nil, tax, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		page                   site.Page
		title, og, description string
	}{
		{site.Page{ID: "home", Title: "首页", URL: "/"}, "站名 & Site", "站名 & Site", "真实简介 <生活>"},
		{site.Page{ID: "home:page:2", Title: "首页", URL: "/page/2/", Pager: site.Pager{Number: 2, Total: 3}}, "站名 & Site · 第 2 页", "站名 & Site · 第 2 页", "真实简介 <生活>"},
		{site.Page{ID: "category:tech", Title: "技术", URL: "/categories/tech/", Pager: site.Pager{Number: 1, Total: 2}}, "技术 · 分类 · 第 1 页 · 站名 & Site", "技术 · 分类 · 第 1 页", "技术介绍"},
		{site.Page{ID: "tag:friends", Title: "友链", URL: "/tags/friends/", Pager: site.Pager{Number: 1, Total: 1}}, "友链 · 标签 · 站名 & Site", "友链 · 标签", "标签介绍"},
		{site.Page{ID: "page:links", Title: "友链", URL: "/links/"}, "友链 · 站名 & Site", "友链", "真实简介 <生活>"},
	} {
		output, e := renderer.Render(&test.page)
		if e != nil {
			t.Fatal(e)
		}
		values, schemas := headValues(t, output)
		if values["title"] != test.title || values["og:title"] != test.og || values["description"] != test.description || values["og:description"] != test.description || values["canonical"] != "https://example.test"+test.page.URL || len(schemas) != 0 {
			t.Fatalf("%s: %#v %v", test.page.ID, values, schemas)
		}
		marker := fmt.Sprintf("第 %d / %d 页", test.page.Pager.Number, test.page.Pager.Total)
		if test.page.Pager.Total > 1 && !strings.Contains(string(output), marker) {
			t.Fatal("visible pagination absent", test.page.ID)
		}
		if test.page.Pager.Total <= 1 && strings.Contains(string(output), `class="list-page-number"`) {
			t.Fatal("single page marker")
		}
	}
}

func TestBlogPostingJSONLDEscapesAndUsesRealMetadata(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c.Site.BaseURL = "https://example.test"
	date := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := date.Add(24 * time.Hour)
	explicit := content.DateText(updated.Format(time.RFC3339))
	item := &content.ContentItem{ID: "post:stable", Kind: content.Post, Format: content.Markdown, Body: "正文内容", Published: &date, Updated: &updated, Meta: content.Metadata{Title: "标题 </script><script>alert(1)</script> & 😀", Summary: "摘要 \"引用\"\n换行", Updated: &explicit, Extra: map[string]any{"cover": "../../media/cover.png"}}}
	p := &site.Page{ID: item.ID, Kind: "post", URL: "/posts/stable/", Title: item.Meta.Title, Visibility: content.Public, Published: &date, Content: item}
	renderer, err := New(c, []*site.Page{p}, &content.Taxonomies{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	output, err := renderer.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	values, schemas := headValues(t, output)
	if len(schemas) != 1 {
		t.Fatal("schema count", len(schemas))
	}
	var posting map[string]any
	if err = json.Unmarshal([]byte(schemas[0]), &posting); err != nil {
		t.Fatal(err, schemas[0])
	}
	for key, value := range map[string]string{"@context": "https://schema.org", "@type": "BlogPosting", "headline": item.Meta.Title, "url": "https://example.test/posts/stable/", "datePublished": date.Format(time.RFC3339), "dateModified": updated.Format(time.RFC3339), "image": "https://example.test/media/cover.png", "description": strings.Join(strings.Fields(item.Meta.Summary), " ")} {
		if posting[key] != value {
			t.Fatal(key, posting[key], value)
		}
	}
	if _, ok := posting["author"]; ok {
		t.Fatal("fabricated author")
	}
	if strings.Contains(schemas[0], "</script>") || values["og:title"] != item.Meta.Title {
		t.Fatal("unsafe head")
	}
	item.Meta.Updated = nil
	delete(item.Meta.Extra, "cover")
	output, err = renderer.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	_, schemas = headValues(t, output)
	json.Unmarshal([]byte(schemas[0]), &posting)
	var optional map[string]any
	json.Unmarshal([]byte(schemas[0]), &optional)
	if optional["dateModified"] != nil || optional["image"] != nil {
		t.Fatal("invented optional metadata")
	}
	for _, visibility := range []content.Status{content.Unlisted, content.Draft} {
		p.Visibility = visibility
		output, e := renderer.Render(p)
		if e != nil {
			t.Fatal(e)
		}
		_, schemas = headValues(t, output)
		if len(schemas) != 0 {
			t.Fatal("nonpublic schema")
		}
	}
}
