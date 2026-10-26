package server

import (
	"net/http/httptest"
	"testing"
)

func TestWildcardAliasesFollowSnapshotAndExactRoutes(t *testing.T) {
	h, _, published, _ := httpFixture(t)
	published.Redirects["/legacy/example/replies-*"] = "/posts/welcome/"
	published.Redirects["/old/*/entry-*/"] = "/posts/welcome/"
	published.Redirects["/legacy/example/replies-special*"] = "/posts/first-steps/"
	published.Redirects["/legacy/example/replies-exact"] = "/about/"
	published.Redirects["/*"] = "/about/"
	published.Redirects["/images/*"] = "/about/"
	published.Redirects["/posts/*/"] = "/about/"
	published.Redirects["/api/*"] = "/about/"
	published.Redirects["/data/*"] = "/about/"
	published.Redirects["/tags/*/"] = "/about/"
	published.Redirects["/feed/*/"] = "/about/"
	published.Redirects["/*/*"] = "/about/"
	published.Redirects["/choice/a*"] = "/posts/welcome/"
	published.Redirects["/choice/*z"] = "/posts/first-steps/"
	published.Redirects["/old/a*"] = "/posts/welcome/"
	published.Redirects["/old/*a*"] = "/posts/first-steps/"
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path     string
		code     int
		location string
	}{
		{"/legacy/example/replies-", 301, "/posts/welcome/"},
		{"/legacy/example/replies-27?q=a%20b", 301, "/posts/welcome/?q=a%20b"},
		{"/legacy/example/replies-text", 301, "/posts/welcome/"},
		{"/legacy/example/replies-special", 301, "/posts/first-steps/"},
		{"/legacy/example/replies-exact", 301, "/about/"},
		{"/choice/az", 301, "/posts/first-steps/"},
		{"/old/a*", 301, "/posts/first-steps/"},
		{"/old/a%2A", 301, "/posts/first-steps/"},
		{"/legacy/example/replies-1/", 404, ""},
		{"/legacy/example/replies-1/more", 404, ""},
		{"/old/sample/entry-/", 301, "/posts/welcome/"},
		{"/old/sample/entry-1", 404, ""},
		{"/posts/welcome/", 200, ""}, {"/images/go.svg", 200, ""},
		{"/feed/atom/", 200, ""}, {"/feed/rss/", 200, ""},
		{"/api/unknown", 404, ""}, {"/tags/unknown/", 404, ""},
		{"/data/unknown", 404, ""}, {"/.secret/file", 404, ""},
		{"/content/file", 404, ""}, {"/a/../file", 404, ""},
	} {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, tc.path, nil))
			if w.Code != tc.code || w.Header().Get("Location") != tc.location {
				t.Errorf("%s %s: %d %q", method, tc.path, w.Code, w.Header().Get("Location"))
			}
			if method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
		}
	}
	published.Redirects["/legacy/example/replies-*"] = "/posts/first-steps/"
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/legacy/example/replies-8", nil))
	if w.Header().Get("Location") != "/posts/first-steps/" {
		t.Fatal("stale target")
	}
	delete(published.Redirects, "/legacy/example/replies-*")
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/legacy/example/replies-8", nil))
	if w.Code != 404 {
		t.Fatal("removed pattern still active")
	}
}

func TestExactAliasDoesNotEnableWildcard(t *testing.T) {
	h, _, published, _ := httpFixture(t)
	published.Redirects["/legacy/example/"] = "/posts/welcome/"
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/legacy/example/replies-8", nil))
	if w.Code != 404 {
		t.Fatal("undeclared wildcard")
	}
}
