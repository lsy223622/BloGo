package site

import (
	"github.com/lsy223622/BloGo/internal/content"
	"testing"
)

func TestWildcardAliasConflicts(t *testing.T) {
	a := routeItem(t, "posts/a.md", "aliases: ['/legacy/example/replies-*']\n")
	b := routeItem(t, "posts/b.md", "aliases: ['/legacy/example/replies-*']\n")
	if err := ValidateRoutes([]*content.ContentItem{a, b}, nil); err == nil {
		t.Fatal("duplicate pattern accepted")
	}
	b.Aliases = []string{"/legacy/example/replies-special*"}
	if err := ValidateRoutes([]*content.ContentItem{a, b}, []string{"legacy/example/replies-static"}); err != nil {
		t.Fatal(err)
	}
	pages := []*Page{{URL: a.URL, Content: a}, {URL: b.URL, Content: b}}
	rules, err := RedirectRules(pages)
	if err != nil || rules[a.Aliases[0]] != a.URL || rules[b.Aliases[0]] != b.URL {
		t.Fatalf("rules: %v %v", rules, err)
	}
	if _, exists := rules[a.Aliases[0]+"/"]; exists {
		t.Fatal("implicit wildcard variant")
	}
	b.Aliases = a.Aliases
	if _, err := RedirectRules(pages); err == nil {
		t.Fatal("conflicting snapshot rules accepted")
	}
	b.Aliases = []string{"/api/*"}
	if err := ValidateRoutes([]*content.ContentItem{b}, nil); err == nil {
		t.Fatal("reserved source accepted")
	}
}
