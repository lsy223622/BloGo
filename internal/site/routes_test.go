package site

import (
	"github.com/lsy223622/BloGo/internal/content"
	"testing"
)

func routeItem(t *testing.T, file, meta string) *content.ContentItem {
	t.Helper()
	i, err := content.Parse(file, []byte("---\n"+meta+"extra: {}\n---\n"))
	if err == nil {
		err = i.ResolveIdentity()
	}
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestRouteConflicts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		metas  []string
		static []string
	}{
		{"same URL", []string{"permalink: /about\n", "permalink: /about/\n"}, nil},
		{"same ID", []string{"id: same\n", "id: same\n"}, nil},
		{"alias canonical", []string{"aliases: [/other]\n", "permalink: /other/\n"}, nil},
		{"alias self", []string{"aliases: [/a]\n"}, nil},
		{"duplicate alias", []string{"aliases: [/old, /old/]\n"}, nil},
		{"alias output ancestor", []string{"aliases: [/old.html]\n", "permalink: /old/html\n"}, []string{"old.html/image.png"}},
		{"static ancestor", []string{"permalink: /about\n"}, []string{"about"}},
		{"static page", []string{"permalink: /about\n"}, []string{"about/index.html"}},
		{"page output directory", []string{"permalink: /about\n"}, []string{"about/index.html/image.png"}},
		{"service mapping", nil, []string{"data/private.json"}},
		{"system static", nil, []string{"rss.xml"}},
		{"reserved asset", []string{"permalink: /assets/custom\n"}, nil},
		{"reserved system", []string{"permalink: /categories\n"}, nil},
		{"reserved pagination", []string{"permalink: /posts/a/page/2\n"}, nil},
		{"reserved alias", []string{"aliases: [/api/events]\n"}, nil},
		{"reserved implicit alias", []string{"aliases: [/categories/tech/index.html]\n"}, nil},
		{"root", []string{"permalink: /\n"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var items []*content.ContentItem
			for j, meta := range tc.metas {
				items = append(items, routeItem(t, []string{"pages/a.md", "pages/b.md"}[j], meta))
			}
			if err := ValidateRoutes(items, tc.static); err == nil {
				t.Fatal("accepted conflict")
			}
		})
	}
}

func TestDistinctRoutesAndAssets(t *testing.T) {
	items := []*content.ContentItem{routeItem(t, "pages/a.md", "permalink: /about\naliases: [/legacy.html]\n"), routeItem(t, "pages/b.md", "permalink: /about/team\n")}
	if err := ValidateRoutes(items, []string{"favicon.ico", "assets/style.css", "images/pic.webp"}); err != nil {
		t.Fatal(err)
	}
}
