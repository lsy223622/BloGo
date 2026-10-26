package site

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

func ApplyNavigation(pages []*Page, index *SiteIndex) {
	positions := map[string]int{}
	for n, item := range index.PublicPosts {
		positions[item.ID] = n
	}
	for _, p := range pages {
		if p.Content == nil || p.Content.Kind != content.Post {
			continue
		}
		n, ok := positions[p.ID]
		if !ok {
			continue
		}
		if n+1 < len(index.PublicPosts) {
			p.Previous = index.PublicPosts[n+1]
		}
		if n > 0 {
			p.Next = index.PublicPosts[n-1]
		}
	}
}

type sitemapURL struct {
	Location string `xml:"loc"`
	Modified string `xml:"lastmod,omitempty"`
}
type sitemapDocument struct {
	XMLName   xml.Name     `xml:"urlset"`
	Namespace string       `xml:"xmlns,attr"`
	URLs      []sitemapURL `xml:"url"`
}

func Sitemap(pages []*Page, baseURL string) ([]byte, error) {
	doc := sitemapDocument{Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, page := range pages {
		if page.Visibility != content.Public || page.NoIndex {
			continue
		}
		entry := sitemapURL{Location: strings.TrimRight(baseURL, "/") + page.URL}
		if page.Content != nil && page.Content.Updated != nil {
			entry.Modified = page.Content.Updated.Format(time.RFC3339)
		}
		doc.URLs = append(doc.URLs, entry)
	}
	sort.Slice(doc.URLs, func(i, j int) bool { return doc.URLs[i].Location < doc.URLs[j].Location })
	data, err := xml.MarshalIndent(doc, "", "  ")
	return append([]byte(xml.Header), data...), err
}

// Redirects emits literal map entries for Nginx's $uri -> canonical-path map.
// The deployment return directive appends the original query string.
func RedirectRules(pages []*Page) (map[string]string, error) {
	canonical := map[string]bool{}
	for _, p := range pages {
		canonical[p.URL] = true
	}
	redirects := map[string]string{}
	add := func(from, to string) error {
		if canonical[from] || from == to {
			return fmt.Errorf("redirect source conflicts with canonical page: %s", from)
		}
		if _, ok := redirects[from]; ok {
			return fmt.Errorf("duplicate redirect source: %s", from)
		}
		redirects[from] = to
		return nil
	}
	for _, p := range pages {
		if p.URL != "/" {
			if err := add(strings.TrimSuffix(p.URL, "/"), p.URL); err != nil {
				return nil, err
			}
		}
		if err := add(p.URL+"index.html", p.URL); err != nil {
			return nil, err
		}
	}
	for _, p := range pages {
		if p.Content == nil {
			continue
		}
		for _, alias := range p.Content.Aliases {
			safe, err := content.AliasURL(alias)
			if err != nil {
				return nil, err
			}
			if err = add(safe, p.URL); err != nil {
				return nil, err
			}
			if strings.HasSuffix(safe, "/") {
				if err = add(strings.TrimSuffix(safe, "/"), p.URL); err != nil {
					return nil, err
				}
				if err = add(safe+"index.html", p.URL); err != nil {
					return nil, err
				}
			}
		}
	}
	return redirects, nil
}

func Redirects(pages []*Page) ([]byte, error) {
	redirects, err := RedirectRules(pages)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, from := range keys(redirects) {
		b.WriteString(strconv.Quote("~^" + regexp.QuoteMeta(from) + "$"))
		b.WriteByte(' ')
		b.WriteString(strconv.Quote(redirects[from]))
		b.WriteString(";\n")
	}
	return []byte(b.String()), nil
}

func Robots(baseURL string) []byte {
	return []byte("User-agent: *\nAllow: /\nSitemap: " + strings.TrimRight(baseURL, "/") + "/sitemap.xml\n")
}
