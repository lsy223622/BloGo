package build

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
	"golang.org/x/net/html"
)

func validateOutput(public string, pages []*site.Page, origin string, strict bool) ([]string, error) {
	files, err := readTree(public, func(file string) bool { return path.Ext(file) == ".html" || staticAllowed(file) })
	if err != nil {
		return nil, err
	}
	expected := map[string]bool{}
	routes := map[string]bool{}
	for _, p := range pages {
		file := content.OutputPath(p.URL)
		expected[file] = true
		routes[p.URL] = true
		routes[p.URL+"index.html"] = true
		if p.URL != "/" {
			routes[strings.TrimSuffix(p.URL, "/")] = true
		}
		if p.Content != nil {
			for _, alias := range p.Content.Aliases {
				routes[alias] = true
				routes[strings.TrimSuffix(alias, "/")] = true
			}
		}
		if _, ok := files[file]; !ok {
			return nil, fmt.Errorf("Manifest page missing output %s", p.ID)
		}
	}
	baseOrigin, err := url.Parse(origin)
	if err != nil {
		return nil, err
	}
	warnings := map[string]bool{}
	for _, file := range fileKeys(files) {
		if path.Ext(file) != ".html" {
			continue
		}
		if !expected[file] {
			return nil, fmt.Errorf("HTML output lacks Manifest entry %s", file)
		}
		base := *baseOrigin
		base.Path = "/" + strings.TrimSuffix(file, "index.html")
		z := html.NewTokenizer(bytes.NewReader(files[file]))
		for {
			kind := z.Next()
			if kind == html.ErrorToken {
				if z.Err() != io.EOF {
					return nil, z.Err()
				}
				break
			}
			if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
				continue
			}
			token := z.Token()
			for _, attr := range token.Attr {
				resource := attr.Key == "src" || attr.Key == "poster" || (token.Data == "link" && attr.Key == "href")
				link := token.Data == "a" && attr.Key == "href"
				if !resource && !link {
					continue
				}
				raw := attr.Val
				ref, err := url.Parse(raw)
				if err != nil {
					return nil, fmt.Errorf("%s: invalid reference %q", file, raw)
				}
				if ref.IsAbs() && (ref.Scheme != baseOrigin.Scheme || ref.Host != baseOrigin.Host) {
					continue
				}
				if ref.Host != "" && ref.Host != baseOrigin.Host {
					continue
				}
				if ref.Path == "" {
					continue
				}
				target := base.ResolveReference(ref)
				if strings.Contains(target.Path, "\\") {
					return nil, fmt.Errorf("%s: invalid resource path", file)
				}
				name := strings.TrimPrefix(target.Path, "/")
				_, isFile := files[name]
				exists := isFile || routes[target.Path]
				if feedFile, _, ok := site.FeedRoute(target.Path); ok {
					_, exists = files[feedFile]
				}
				if !exists {
					message := fmt.Sprintf("%s: missing %s", file, raw)
					if resource || strict {
						return nil, fmt.Errorf("%s", message)
					}
					warnings[message] = true
				}
			}
		}
	}
	result := make([]string, 0, len(warnings))
	for warning := range warnings {
		result = append(result, warning)
	}
	sort.Strings(result)
	return result, nil
}
