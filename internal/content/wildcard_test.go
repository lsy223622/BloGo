package content

import "testing"

func TestWildcardAliasSyntax(t *testing.T) {
	for _, raw := range []string{"/legacy/example/replies-*", "/legacy/*/entry-*/", "/old-*.html"} {
		got, err := AliasURL(raw)
		if err != nil || got != raw {
			t.Fatalf("%q: %q %v", raw, got, err)
		}
	}
	for _, raw := range []string{"/legacy/**", "/legacy/a?", "/legacy/[ab]", "//example.test/*", "/legacy/../*", "/legacy/%2e/*", "/legacy/*?q=1", "/legacy/*#x", "/private/*", "/.hidden/*", "/content/*"} {
		if _, err := AliasURL(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
