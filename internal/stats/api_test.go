package stats

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
)

func apiFixture(t *testing.T) (*Handler, *Store, *config.Config, *site.Manifest) {
	t.Helper()
	s, _ := openStore(t)
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c.Site.BaseURL = "https://blog.example"
	c.Stats.ManifestPath = filepath.Join(t.TempDir(), "manifest.json")
	date := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	m := &site.Manifest{SchemaVersion: 1, BuildID: "build-test", Fingerprint: strings.Repeat("a", 64), GeneratedAt: date, Pages: []site.PageRecord{{ID: "home", Kind: "system", URL: "/", Title: "Home", Visibility: content.Public, StatsEnabled: true}, {ID: "post:a", Kind: "post", URL: "/posts/a/", Title: "A", Visibility: content.Public, StatsEnabled: true, Published: &date}, {ID: "page:secret", Kind: "page", URL: "/secret/", Title: "Secret", Visibility: content.Unlisted, StatsEnabled: true}, {ID: "page:disabled", Kind: "page", URL: "/disabled/", Title: "Disabled", Visibility: content.Public, StatsEnabled: false}, {ID: "category:tech", Kind: "category", URL: "/categories/tech/", Title: "Tech", Visibility: content.Public, StatsEnabled: true}}}
	saveManifest(t, c.Stats.ManifestPath, m)
	h, err := NewHandler(c, s, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	h.now = func() time.Time { return date }
	return h, s, c, m
}
func saveManifest(t *testing.T, path string, m *site.Manifest) {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func eventBody(eventID, page, source string) string {
	data, _ := json.Marshal(map[string]any{"v": 1, "event_id": strings.Repeat(eventID, 32), "page_id": page, "visitor_id": strings.Repeat("b", 32), "referrer_host": source})
	return string(data)
}
func sendEvent(h *Handler, body, origin, referer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	return response
}

func TestEventAPIDedupAndNavigationSource(t *testing.T) {
	h, s, _, _ := apiFixture(t)
	body := eventBody("a", "post:a", "Search.Example")
	for n := 0; n < 2; n++ {
		response := sendEvent(h, body, "https://blog.example", "https://blog.example/posts/a/?x=1")
		if response.Code != 204 {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
	}
	response := sendEvent(h, eventBody("c", "category:tech", "blog.example"), "", "")
	if response.Code != 204 {
		t.Fatal(response.Body)
	}
	response = sendEvent(h, eventBody("d", "page:secret", ""), "", "")
	if response.Code != 204 {
		t.Fatal("unlisted denied")
	}
	var pv, uv int
	if err := s.db.QueryRow("SELECT pv,uv FROM daily_site_stats").Scan(&pv, &uv); err != nil || pv != 3 || uv != 1 {
		t.Fatalf("counts %d %d: %v", pv, uv, err)
	}
	var source string
	if err := s.db.QueryRow("SELECT source FROM daily_referrers WHERE page_id='post:a'").Scan(&source); err != nil || source != "search.example" {
		t.Fatalf("Referer confused with navigation source: %s %v", source, err)
	}
}

func TestEventAPIRejectsMalformedAndUnwantedFields(t *testing.T) {
	h, _, _, _ := apiFixture(t)
	valid := eventBody("a", "post:a", "")
	for _, bad := range []string{strings.Replace(valid, `"v":1`, `"v":2`, 1), strings.Replace(valid, `"v":1`, `"v":"1"`, 1), strings.Replace(valid, `"referrer_host":"",`, "", 1), strings.Replace(valid, `"visitor_id":"`+strings.Repeat("b", 32)+`"`, `"visitor_id":null`, 1), `{"v":1,"v":1}`, valid[:len(valid)-1] + `,"date":"2026-10-23"}`, valid[:len(valid)-1] + `,"ip":"127.0.0.1"}`, valid[:len(valid)-1] + `,"url":"/a/"}`, eventBody("A", "post:a", ""), eventBody("a", "../../etc/passwd", ""), eventBody("a", "page:disabled", ""), eventBody("a", "post:a", "https://search.example/path"), eventBody("a", "post:a", "host.example:443")} {
		if response := sendEvent(h, bad, "", ""); response.Code != 400 {
			t.Errorf("accepted %s: %d %s", bad, response.Code, response.Body)
		}
	}
	if response := sendEvent(h, strings.Repeat("x", 2049), "", ""); response.Code != 413 {
		t.Fatal("oversized body accepted")
	}
	if response := sendEvent(h, valid, "https://evil.example", ""); response.Code != 400 {
		t.Fatal("cross-origin accepted")
	}
	if response := sendEvent(h, valid, "", "https://blog.example/categories/tech/"); response.Code != 400 {
		t.Fatal("known page mismatch accepted")
	}
	req := httptest.NewRequest("POST", "/api/events", strings.NewReader(valid))
	req.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	if response.Code != 415 {
		t.Fatal("wrong media accepted")
	}
	response = httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("OPTIONS", "/api/events", nil))
	if response.Code != 405 || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS/method")
	}
}

func TestManifestReloadAndControlledFailure(t *testing.T) {
	h, s, c, m := apiFixture(t)
	m.Pages[1].URL = "/renamed/"
	saveManifest(t, c.Stats.ManifestPath, m)
	if response := sendEvent(h, eventBody("a", "post:a", ""), "", "https://blog.example/posts/a/"); response.Code != 204 {
		t.Fatal("stable ID old browser rejected")
	}
	m.Pages = append(m.Pages[:1], m.Pages[2:]...)
	saveManifest(t, c.Stats.ManifestPath, m)
	if response := sendEvent(h, eventBody("c", "post:a", ""), "", ""); response.Code != 400 {
		t.Fatal("deleted page accepted")
	}
	s.Close()
	response := sendEvent(h, eventBody("d", "home", ""), "", "")
	if response.Code != 503 || strings.Contains(response.Body.String(), "sql") || strings.Contains(response.Body.String(), ".db") {
		t.Fatal("database error not controlled")
	}
}

func TestSourcesAndRateLimit(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"", "direct"}, {"BLOG.EXAMPLE.", "internal"}, {"例子.测试", "xn--fsqu00a.xn--0zwm56d"}, {"[2001:db8::1]", "2001:db8::1"}, {"192.0.2.1", "192.0.2.1"}} {
		source, err := NormalizeSource(tc.input, "blog.example")
		if err != nil || source != tc.want {
			t.Fatalf("%s => %s %v", tc.input, source, err)
		}
	}
	for _, bad := range []string{"a/b", "a@b", "a:443", " a.example", "-bad.example", "bad..example", strings.Repeat("a", 64) + ".example", "host?x", "host\x00name"} {
		if _, err := NormalizeSource(bad, "blog.example"); err == nil {
			t.Errorf("accepted host %q", bad)
		}
	}
	h, _, _, _ := apiFixture(t)
	h.limit.tokens = 0
	h.limit.last = h.now()
	if response := sendEvent(h, eventBody("a", "home", ""), "", ""); response.Code != 429 {
		t.Fatal("rate limit absent")
	}
}
