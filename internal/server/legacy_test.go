package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lsy223622/BloGo/internal/site"
)

func TestLegacyCommentPagesUseCurrentPostAliases(t *testing.T) {
	h, _, published, _ := httpFixture(t)
	published.Redirects["/archives/69/"] = "/posts/welcome/"
	published.Redirects["/archives/70/"] = "/about/"
	published.Redirects["/archives/69/comment-page-2"] = "/about/"
	exact := "/archives/69/comment-page-3/"
	published.Manifest.Pages = append(published.Manifest.Pages, site.PageRecord{ID: "page:exact", Kind: "page", URL: exact})
	file := filepath.Join(published.Output, "public/archives/69/comment-page-3/index.html")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("exact page"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path     string
		code     int
		location string
	}{
		{"/archives/69/comment-page-1", 301, "/posts/welcome/"},
		{"/archives/69/comment-page-123/?x=a%20b", 301, "/posts/welcome/?x=a%20b"},
		{"/archives/999/comment-page-1", 404, ""}, {"/archives/70/comment-page-1", 404, ""},
		{"/archives/69/comment-page-0", 404, ""}, {"/archives/69/comment-page-01", 404, ""},
		{"/archives/69/comment-page-x", 404, ""}, {"/archives/69/comment-page-1/extra", 404, ""},
		{"/archives/69/comment-page-2", 301, "/about/"}, {exact, 200, ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", test.path, nil))
		if w.Code != test.code || w.Header().Get("Location") != test.location {
			t.Fatalf("%s: %d %s", test.path, w.Code, w.Header().Get("Location"))
		}
	}
	published.Redirects["/archives/69/"] = "/posts/first-steps/"
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("HEAD", "/archives/69/comment-page-9/", nil))
	if w.Code != 301 || w.Header().Get("Location") != "/posts/first-steps/" || w.Body.Len() != 0 {
		t.Fatal("updated redirect/HEAD", w.Code, w.Header(), w.Body)
	}
	delete(published.Redirects, "/archives/69/")
	if err := h.Update(published); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/archives/69/comment-page-9/", nil))
	if w.Code != 404 {
		t.Fatal("removed alias remains active")
	}
}
