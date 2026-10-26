package site

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/markdown"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const contentNamespace = "http://purl.org/rss/1.0/modules/content/"
const rdfNamespace = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
const rssNamespace = "http://purl.org/rss/1.0/"
const atomNamespace = "http://www.w3.org/2005/Atom"

type FeedSet struct{ RSS2, RDF, Atom []byte }
type feedEntry struct {
	ID, URL, Title, Summary, HTML string
	Published, Updated            time.Time
}
type rssGUID struct {
	PermaLink bool   `xml:"isPermaLink,attr"`
	ID        string `xml:",chardata"`
}
type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	Date        string  `xml:"pubDate"`
	Description string  `xml:"description"`
	Content     string  `xml:"content:encoded"`
}
type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}
type rssDocument struct {
	XMLName          xml.Name   `xml:"rss"`
	Version          string     `xml:"version,attr"`
	ContentNamespace string     `xml:"xmlns:content,attr"`
	Channel          rssChannel `xml:"channel"`
}
type rdfResource struct {
	URL string `xml:"rdf:resource,attr"`
}
type rdfChannel struct {
	About       string        `xml:"rdf:about,attr"`
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	Items       []rdfResource `xml:"items>rdf:Seq>rdf:li"`
}
type rdfItem struct {
	About       string `xml:"rdf:about,attr"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Date        string `xml:"dc:date"`
	Description string `xml:"description"`
	Content     string `xml:"content:encoded"`
}
type rdfDocument struct {
	XMLName xml.Name   `xml:"rdf:RDF"`
	RDF     string     `xml:"xmlns:rdf,attr"`
	RSS     string     `xml:"xmlns,attr"`
	Content string     `xml:"xmlns:content,attr"`
	DC      string     `xml:"xmlns:dc,attr"`
	Channel rdfChannel `xml:"channel"`
	Items   []rdfItem  `xml:"item"`
}
type atomText struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}
type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	URL  string `xml:"href,attr"`
}
type atomAuthor struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}
type atomEntry struct {
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Link      atomLink `xml:"link"`
	Published string   `xml:"published"`
	Updated   string   `xml:"updated"`
	Summary   atomText `xml:"summary"`
	Content   atomText `xml:"content"`
}
type atomDocument struct {
	XMLName   xml.Name    `xml:"feed"`
	Namespace string      `xml:"xmlns,attr"`
	ID        string      `xml:"id"`
	Title     string      `xml:"title"`
	Updated   string      `xml:"updated"`
	Author    atomAuthor  `xml:"author"`
	Links     []atomLink  `xml:"link"`
	Entries   []atomEntry `xml:"entry"`
}

func Feed(index *SiteIndex, baseURL, title string, maxSummary int) ([]byte, error) {
	feeds, err := Feeds(index, baseURL, title, maxSummary)
	return feeds.RSS2, err
}

func Feeds(index *SiteIndex, baseURL, title string, maxSummary int) (FeedSet, error) {
	now := time.Now()
	baseURL = strings.TrimRight(baseURL, "/")
	entries := []feedEntry{}
	for _, item := range index.PublicPosts {
		if item.Kind != content.Post || item.Status != content.Public {
			continue
		}
		if item.Published == nil {
			return FeedSet{}, fmt.Errorf("%s: public post lacks publication date", item.ID)
		}
		if item.Published.After(now) {
			continue
		}
		body, summary := item.Body, item.Meta.Summary
		if item.Format == content.Markdown {
			converted, err := markdown.Convert(item.Body, summary, maxSummary)
			if err != nil {
				return FeedSet{}, err
			}
			body, summary = converted.HTML, converted.Summary
		}
		full, err := absoluteFeedBody(body, baseURL+item.URL)
		if err != nil {
			return FeedSet{}, err
		}
		updated := *item.Published
		if item.Updated != nil {
			updated = *item.Updated
		}
		entries = append(entries, feedEntry{item.ID, baseURL + item.URL, item.Meta.Title, summary, full, *item.Published, updated})
	}
	rss := rssDocument{Version: "2.0", ContentNamespace: contentNamespace, Channel: rssChannel{Title: title, Link: baseURL + "/", Description: title}}
	rdf := rdfDocument{RDF: rdfNamespace, RSS: rssNamespace, Content: contentNamespace, DC: "http://purl.org/dc/elements/1.1/", Channel: rdfChannel{About: baseURL + "/feed/rss/", Title: title, Link: baseURL + "/", Description: title}}
	atomFeed := atomDocument{Namespace: atomNamespace, ID: baseURL + "/feed/atom/", Title: title, Author: atomAuthor{title, baseURL + "/"}, Links: []atomLink{{"self", "application/atom+xml", baseURL + "/feed/atom/"}, {"alternate", "text/html", baseURL + "/"}}}
	var latest time.Time
	for _, entry := range entries {
		// A fragment based on the stable ID survives a later slug/permalink change.
		identity := (&url.URL{Fragment: entry.ID}).String()
		identity = baseURL + "/" + identity
		rss.Channel.Items = append(rss.Channel.Items, rssItem{entry.Title, entry.URL, rssGUID{ID: entry.ID}, entry.Published.Format(time.RFC1123Z), entry.Summary, entry.HTML})
		rdf.Channel.Items = append(rdf.Channel.Items, rdfResource{identity})
		rdf.Items = append(rdf.Items, rdfItem{identity, entry.Title, entry.URL, entry.Published.Format(time.RFC3339), entry.Summary, entry.HTML})
		atomFeed.Entries = append(atomFeed.Entries, atomEntry{identity, entry.Title, atomLink{"alternate", "text/html", entry.URL}, entry.Published.Format(time.RFC3339), entry.Updated.Format(time.RFC3339), atomText{"text", entry.Summary}, atomText{"html", entry.HTML}})
		if entry.Updated.After(latest) {
			latest = entry.Updated
		}
	}
	// Empty feeds still need an Atom feed-level timestamp; no article date is invented.
	if latest.IsZero() {
		latest = now.UTC()
	}
	atomFeed.Updated = latest.Format(time.RFC3339)
	encode := func(value any) ([]byte, error) {
		data, err := xml.MarshalIndent(value, "", "  ")
		return append([]byte(xml.Header), data...), err
	}
	var result FeedSet
	var err error
	if result.RSS2, err = encode(rss); err != nil {
		return result, err
	}
	if result.RDF, err = encode(rdf); err != nil {
		return result, err
	}
	result.Atom, err = encode(atomFeed)
	return result, err
}

func absoluteFeedBody(body, articleURL string) (string, error) {
	base, err := url.Parse(articleURL)
	if err != nil {
		return "", err
	}
	nodes, err := html.ParseFragment(strings.NewReader(body), &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"})
	if err != nil {
		return "", err
	}
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		for i, attribute := range node.Attr {
			switch attribute.Key {
			case "srcset":
				node.Attr[i].Val = absoluteSrcset(attribute.Val, base)
			case "href", "src", "poster", "cite":
				reference, err := url.Parse(attribute.Val)
				if err == nil {
					node.Attr[i].Val = base.ResolveReference(reference).String()
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	var output bytes.Buffer
	for _, node := range nodes {
		visit(node)
		if err = html.Render(&output, node); err != nil {
			return "", err
		}
	}
	return output.String(), nil
}

func absoluteSrcset(value string, base *url.URL) string {
	space := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
	var output strings.Builder
	for position := 0; position < len(value); {
		start := position
		for position < len(value) && (space(value[position]) || value[position] == ',') {
			position++
		}
		output.WriteString(value[start:position])
		if position == len(value) {
			break
		}
		start = position
		// Srcset URL tokens end at HTML whitespace, not at embedded commas.
		// Trailing commas terminate a candidate; data-URL commas remain intact.
		for position < len(value) && !space(value[position]) {
			position++
		}
		end := position
		for end > start && value[end-1] == ',' {
			end--
		}
		raw := value[start:end]
		if reference, err := url.Parse(raw); err == nil && !reference.IsAbs() {
			output.WriteString(base.ResolveReference(reference).String())
		} else {
			output.WriteString(raw)
		}
		output.WriteString(value[end:position])
		if end != position {
			continue
		}
		// Keep descriptors verbatim, including parenthesized parsing-error text.
		start = position
		parentheses := 0
		for position < len(value) {
			c := value[position]
			position++
			if c == '(' {
				parentheses++
			} else if c == ')' && parentheses > 0 {
				parentheses--
			}
			if c == ',' && parentheses == 0 {
				break
			}
		}
		output.WriteString(value[start:position])
	}
	return output.String()
}

// FeedRoute maps only the three fixed legacy entrances and the primary feed.
func FeedRoute(path string) (file, mediaType string, ok bool) {
	switch path {
	case "/rss.xml", "/feed", "/feed/":
		return "rss.xml", "application/rss+xml; charset=utf-8", true
	case "/feed/rss", "/feed/rss/":
		return "feed/rss/index.xml", "application/rdf+xml; charset=utf-8", true
	case "/feed/atom", "/feed/atom/":
		return "feed/atom/index.xml", "application/atom+xml; charset=utf-8", true
	}
	return "", "", false
}
