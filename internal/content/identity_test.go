package content

import (
	"strings"
	"testing"
)

func identity(t *testing.T, source, meta string) *ContentItem {
	t.Helper()
	item, err := Parse(source, []byte("---\n"+meta+"---\nBody"))
	if err != nil {
		t.Fatal(err)
	}
	if err = item.ResolveIdentity(); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestIdentityAndDefaults(t *testing.T) {
	a := identity(t, "posts/Title___-- Name.md", "id: stable\nslug: first\n")
	b := identity(t, "posts/other.md", "id: stable\nslug: second\n")
	if a.ID != "post:stable" || a.ID != b.ID || a.URL == b.URL || a.OutputPath != "posts/first/index.html" {
		t.Fatalf("identity: %+v %+v", a, b)
	}
	c := identity(t, "posts/Title___-- Name.md", "title: 不参与 slug\n")
	if c.Slug != "title-name" || c.ID != "post:title-name" {
		t.Fatalf("slug: %+v", c)
	}
	p := identity(t, "pages/guide/part-two.md", "title: 第二章\n")
	if p.URL != "/guide/part-two/" || p.ID != "page:part-two" {
		t.Fatalf("nested: %+v", p)
	}
	p = identity(t, "pages/中文/目录.md", "slug: ascii\npermalink: /guide/intro\naliases: [/old.html, /old]\n")
	if p.URL != "/guide/intro/" || p.Aliases[0] != "/old.html" || p.Aliases[1] != "/old/" {
		t.Fatalf("permalink/aliases: %+v", p)
	}
}

func TestInvalidIdentities(t *testing.T) {
	for _, tc := range []struct{ source, meta string }{
		{"posts/中文.md", ""}, {"posts/a.md", "id: Upper\n"}, {"posts/a.md", "id: ''\n"}, {"posts/a.md", "slug: ''\n"}, {"posts/a.md", "slug: a--b\n"},
		{"pages/Invalid/a.md", ""}, {"pages/a.md", "permalink: /a.html\n"}, {"pages/a.md", "permalink: /a//b\n"}, {"pages/a.md", "permalink: /a/../b\n"}, {"pages/a.md", "permalink: /a%2fb\n"}, {"pages/a.md", "permalink: /a?q=1\n"}, {"pages/a.md", "permalink: /a#x\n"}, {"pages/a.md", "permalink: '//example.com/a'\n"},
		{"pages/a.md", "aliases: ['/a;return-200']\n"}, {"pages/a.md", "aliases: ['/a%2fb']\n"}, {"pages/a.md", "aliases: ['/old.html?q=1']\n"},
	} {
		t.Run(tc.source+tc.meta, func(t *testing.T) {
			item, err := Parse(tc.source, []byte("---\n"+tc.meta+"extra: {}\n---\n"))
			if err == nil {
				err = item.ResolveIdentity()
			}
			if err == nil || !strings.Contains(err.Error(), tc.source) {
				t.Fatalf("accepted %s: %s, %v", tc.source, tc.meta, err)
			}
		})
	}
}

func TestCanonicalRoot(t *testing.T) {
	item := identity(t, "pages/home.md", "permalink: /\n")
	if item.URL != "/" || item.OutputPath != "index.html" {
		t.Fatalf("%+v", item)
	}
}
