package site

import (
	"encoding/xml"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

const testRSS = "http://purl.org/rss/1.0/"
const testRDF = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
const testContent = "http://purl.org/rss/1.0/modules/content/"
const testAtom = "http://www.w3.org/2005/Atom"

type feedNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Text     string     `xml:",chardata"`
	Children []feedNode `xml:",any"`
}

func (n feedNode) nodes(space, name string) []feedNode {
	var result []feedNode
	for _, child := range n.Children {
		if child.XMLName.Space == space && child.XMLName.Local == name {
			result = append(result, child)
		}
	}
	return result
}
func (n feedNode) one(t *testing.T, space, name string) feedNode {
	t.Helper()
	nodes := n.nodes(space, name)
	if len(nodes) != 1 {
		t.Fatalf("%s {%s}: %d", name, space, len(nodes))
	}
	return nodes[0]
}
func (n feedNode) attr(space, name string) string {
	for _, attribute := range n.Attrs {
		if attribute.Name.Space == space && attribute.Name.Local == name {
			return attribute.Value
		}
	}
	return ""
}
func parseFeed(t *testing.T, data []byte) feedNode {
	t.Helper()
	var root feedNode
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatal(err, string(data))
	}
	return root
}

func TestFullFeedsFormatsVisibilityAndStableIDs(t *testing.T) {
	published := time.Date(2020, 2, 3, 4, 5, 6, 0, time.UTC)
	updated := published.Add(48 * time.Hour)
	future := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	post := indexedItem("stable", content.Post, content.Public, &published, &updated)
	post.URL = "/posts/stable/"
	post.Meta.Title = "中文 & <标题> 😀"
	post.Meta.Summary = "摘要 & < > 😀"
	post.Body = "## 正文\n\n[相对](../other/?x=1&y=2) [锚点](#章节) [外链](https://other.test/a)\n\n![图片](/media/photo.png)\n\n```text\n]]> <hello> &\n```"
	older := indexedItem("older", content.Post, content.Public, &published, nil)
	older.Body = "旧文章"
	older.Format = content.HTML
	older.Body = `<p>HTML <a href="./child/">link</a><img src="//cdn.test/a.png"></p>`
	hidden := indexedItem("hidden", content.Post, content.Unlisted, &published, &updated)
	draft := indexedItem("draft", content.Post, content.Draft, &published, &updated)
	upcoming := indexedItem("future", content.Post, content.Public, &future, &future)
	page := indexedItem("page", content.Page, content.Public, nil, nil)
	index := &SiteIndex{PublicPosts: []*content.ContentItem{post, older, hidden, draft, upcoming, page}}
	feeds, err := Feeds(index, "https://example.test", "站名 & Site", 160)
	if err != nil {
		t.Fatal(err)
	}
	checkBody := func(body string) {
		t.Helper()
		for _, value := range []string{`href="https://example.test/posts/other/?x=1&amp;y=2"`, `href="https://example.test/posts/stable/#%E7%AB%A0%E8%8A%82"`, `src="https://example.test/media/photo.png"`, `href="https://other.test/a"`, "正文", "chroma"} {
			if !strings.Contains(body, value) {
				t.Fatalf("body missing %s: %s", value, body)
			}
		}
		if strings.Contains(body, "data-page-id") || strings.Contains(body, "/__blogo/") {
			t.Fatal("page shell/script in feed")
		}
	}
	rss := parseFeed(t, feeds.RSS2)
	if rss.XMLName.Local != "rss" || rss.attr("", "version") != "2.0" {
		t.Fatal("RSS2 root")
	}
	items := rss.one(t, "", "channel").nodes("", "item")
	if len(items) != 2 {
		t.Fatal("visibility/order", len(items))
	}
	if items[0].one(t, "", "guid").Text != post.ID || items[0].one(t, "", "description").Text != post.Meta.Summary || items[0].one(t, "", "title").Text != post.Meta.Title {
		t.Fatal("RSS2 identity/text")
	}
	checkBody(items[0].one(t, testContent, "encoded").Text)
	if strings.Index(string(feeds.RSS2), "<description>摘要") > strings.Index(string(feeds.RSS2), "<content:encoded>") {
		t.Fatal("full content must follow summary")
	}
	rdf := parseFeed(t, feeds.RDF)
	if rdf.XMLName != (xml.Name{Space: testRDF, Local: "RDF"}) {
		t.Fatal("RDF root", rdf.XMLName)
	}
	rdfItems := rdf.nodes(testRSS, "item")
	if len(rdfItems) != 2 {
		t.Fatal("RDF items")
	}
	checkBody(rdfItems[0].one(t, testContent, "encoded").Text)
	sequence := rdf.one(t, testRSS, "channel").one(t, testRSS, "items").one(t, testRDF, "Seq").nodes(testRDF, "li")
	if len(sequence) != 2 || sequence[0].attr(testRDF, "resource") != rdfItems[0].attr(testRDF, "about") {
		t.Fatal("RDF ordered membership")
	}
	atom := parseFeed(t, feeds.Atom)
	if atom.XMLName != (xml.Name{Space: testAtom, Local: "feed"}) {
		t.Fatal("Atom root")
	}
	if atom.one(t, testAtom, "author").one(t, testAtom, "name").Text != "站名 & Site" {
		t.Fatal("real site identity")
	}
	entries := atom.nodes(testAtom, "entry")
	if len(entries) != 2 {
		t.Fatal("Atom visibility")
	}
	checkBody(entries[0].one(t, testAtom, "content").Text)
	atomID := entries[0].one(t, testAtom, "id").Text
	if id, e := url.Parse(atomID); e != nil || !id.IsAbs() {
		t.Fatal("Atom ID", atomID)
	}
	if entries[0].one(t, testAtom, "updated").Text != updated.Format(time.RFC3339) || entries[1].one(t, testAtom, "updated").Text != published.Format(time.RFC3339) {
		t.Fatal("invented update date")
	}
	if !strings.Contains(entries[1].one(t, testAtom, "content").Text, `https://cdn.test/a.png`) {
		t.Fatal("protocol relative image")
	}
	post.URL = "/posts/renamed/"
	next, err := Feeds(index, "https://example.test", "站名 & Site", 160)
	if err != nil {
		t.Fatal(err)
	}
	if parseFeed(t, next.Atom).nodes(testAtom, "entry")[0].one(t, testAtom, "id").Text != atomID {
		t.Fatal("slug rename changed Atom identity")
	}
}

func TestEmptyFeedsRemainValid(t *testing.T) {
	feeds, err := Feeds(&SiteIndex{}, "https://example.test", "Empty", 160)
	if err != nil {
		t.Fatal(err)
	}
	parseFeed(t, feeds.RSS2)
	parseFeed(t, feeds.RDF)
	atom := parseFeed(t, feeds.Atom)
	if _, err = time.Parse(time.RFC3339, atom.one(t, testAtom, "updated").Text); err != nil {
		t.Fatal(err)
	}
}
