package render

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// PrepareStandalone edits only the actual body terminator. Token offsets refer
// to the original bytes; author formatting and scripts are never reserialized.
func PrepareStandalone(body string, noIndex bool, script string) ([]byte, error) {
	z := html.NewTokenizer(strings.NewReader(body))
	offset, bodyEnd := 0, -1
	inHead, hasNoIndex, hasBody := false, false, false
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			break
		}
		raw := z.Raw()
		start := offset
		offset += len(raw)
		token := z.Token()
		if kind == html.StartTagToken && token.Data == "head" {
			inHead = true
		}
		if kind == html.EndTagToken && token.Data == "head" {
			inHead = false
		}
		if kind == html.StartTagToken && token.Data == "body" {
			hasBody = true
		}
		if kind == html.EndTagToken && token.Data == "body" && hasBody {
			if bodyEnd >= 0 {
				return nil, fmt.Errorf("standalone has multiple body terminators")
			}
			bodyEnd = start
		}
		if inHead && (kind == html.StartTagToken || kind == html.SelfClosingTagToken) && token.Data == "meta" {
			name, value := "", ""
			for _, a := range token.Attr {
				if a.Key == "name" {
					name = strings.ToLower(a.Val)
				}
				if a.Key == "content" {
					value = strings.ToLower(a.Val)
				}
			}
			if name == "robots" {
				for _, directive := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
					if directive == "noindex" || directive == "none" {
						hasNoIndex = true
					}
				}
			}
		}
	}
	if noIndex && !hasNoIndex {
		return nil, fmt.Errorf("unlisted standalone requires robots noindex in head")
	}
	if script == "" {
		return []byte(body), nil
	}
	if bodyEnd < 0 {
		return nil, fmt.Errorf("standalone tracking requires a real closing body tag")
	}
	return []byte(body[:bodyEnd] + script + body[bodyEnd:]), nil
}
