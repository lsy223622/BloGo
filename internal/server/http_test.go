package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/stats"
)

func httpFixture(t *testing.T) (*Handler, *config.Config, *build.Result, *stats.Store) {
	t.Helper()
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	published, err := build.Build(c, filepath.Join(t.TempDir(), "site"), false)
	if err != nil {
		t.Fatal(err)
	}
	c.Stats.ManifestPath = filepath.Join(published.Output, "meta/manifest.json")
	c.Stats.DatabasePath = filepath.Join(t.TempDir(), "stats.db")
	c.Stats.PublicOutput = filepath.Join(t.TempDir(), "stats.json")
	c.Stats.Details.Enabled = true
	store, err := stats.OpenWriter(context.Background(), c.Stats.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	collector, err := stats.NewHandler(c, store, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(c, published, collector)
	if err != nil {
		t.Fatal(err)
	}
	return h, c, published, store
}

func TestPublicHTTPRoutes(t *testing.T) {
	h, c, published, store := httpFixture(t)
	for _, test := range []struct {
		path, method string
		code         int
		location     string
	}{
		{"/", "GET", 200, ""}, {"/", "HEAD", 200, ""}, {"/posts/welcome/", "GET", 200, ""}, {"/images/go.svg", "GET", 200, ""},
		{"/404/", "GET", 404, ""}, {"/missing/", "GET", 404, ""}, {"/posts/welcome?x=1", "GET", 301, "/posts/welcome/?x=1"},
		{"/posts/welcome/index.html?q=1", "GET", 301, "/posts/welcome/?q=1"}, {"/Legacy.html", "GET", 301, "/posts/welcome/"}, {"/", "POST", 405, ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(test.method, test.path, nil))
		if w.Code != test.code || w.Header().Get("Location") != test.location {
			t.Fatalf("%s: %d %s", test.path, w.Code, w.Body)
		}
		if test.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
	hashes, err := filepath.Glob(filepath.Join(published.Output, "public/assets/theme.*.css"))
	if err != nil || len(hashes) == 0 {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/assets/"+filepath.Base(hashes[0]), nil))
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("hashed cache", w.Header())
	}
	published.Redirects["/new-alias/"] = "/posts/welcome/"
	if err = h.Update(published); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/new-alias/", nil))
	if w.Code != 301 {
		t.Fatal("alias update missing")
	}
	body := `{"v":1,"event_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","page_id":"post:welcome","visitor_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","referrer_host":""}`
	r := httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:10"
	r.Header.Set("X-Forwarded-For", "192.0.2.3")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	events, count, err := store.Details(context.Background())
	if err != nil || count != 1 || events[0].IP != "192.0.2.3" {
		t.Fatal("trusted metadata not passed", events, count, err)
	}
	report := stats.PublicReport{SchemaVersion: 1, BuildID: published.Manifest.BuildID, Pages: map[string]stats.PageViews{"post:welcome": {Views: 1}, "page:lab": {Views: 9}}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(c.Stats.PublicOutput, data, 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/data/stats.json", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "page:lab") {
		t.Fatal("public report filtering", w.Code, w.Body)
	}
}

func TestPrivatePathsNotServed(t *testing.T) {
	h, c, published, _ := httpFixture(t)
	for _, path := range []string{"/config.yaml", "/content/site.yaml", "/data/", "/data/statistics/stats.db", "/data/private/report.html", "/meta/manifest.json", "/statistics/visitor.key", "/templates/page.html", "/media/../config.yaml", "/%2e%2e/config.yaml", "/assets/", "/__blogo/revision"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("private path %s: %d", path, w.Code)
		}
	}
	c.Stats.Enabled = false
	if err := h.Reconfigure(c, published, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/data/stats.json", "/api/events"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("disabled endpoint", path, w.Code)
		}
	}
}
