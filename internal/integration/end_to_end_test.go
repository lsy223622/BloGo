package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/stats"
	"golang.org/x/net/html"
)

func TestSiteAndStatisticsAcrossContentRevisions(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"content", "templates", "static"} {
		must(os.CopyFS(filepath.Join(root, name), os.DirFS(filepath.Join("../..", name))))
	}
	tax, err := os.ReadFile("../../taxonomies.yaml")
	must(err)
	must(os.WriteFile(filepath.Join(root, "taxonomies.yaml"), tax, 0644))
	configFile := filepath.Join(root, "config.yaml")
	must(os.WriteFile(configFile, []byte("site:\n  title: Integration\n  base_url: https://blog.example\npaths:\n  source: content\n  templates: templates\n  static: static\n  taxonomies: taxonomies.yaml\nbuild:\n  page_size: 1\n"), 0644))
	c, err := config.Load(configFile)
	must(err)
	out := filepath.Join(root, "output")
	result, err := build.Build(c, out, false)
	must(err)
	var oldIDs []string
	for _, p := range result.Manifest.Pages {
		oldIDs = append(oldIDs, p.ID)
	}
	for _, id := range []string{"home:page:2", "category:tech:page:2", "tag:esp32:page:2", "archive:2026-10:page:2", "post:first-steps", "post:release-notes", "post:review-copy"} {
		if !strings.Contains(strings.Join(oldIDs, "|"), id) {
			t.Fatalf("missing identity %s", id)
		}
	}
	var feed struct {
		Channel struct {
			Items []struct {
				ID   string `xml:"guid"`
				Link string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	data, err := os.ReadFile(filepath.Join(out, "public/rss.xml"))
	must(err)
	must(xml.Unmarshal(data, &feed))
	if len(feed.Channel.Items) != 3 || feed.Channel.Items[0].ID != "post:welcome" {
		t.Fatalf("feed order/count %s", data)
	}
	var sitemap struct {
		URLs []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
	}
	data, err = os.ReadFile(filepath.Join(out, "public/sitemap.xml"))
	must(err)
	must(xml.Unmarshal(data, &sitemap))
	urls := map[string]bool{}
	for _, u := range sitemap.URLs {
		urls[u.Location] = true
	}
	for _, p := range result.Manifest.Pages {
		if p.Visibility == content.Draft {
			t.Fatal("production draft")
		}
		public := p.Visibility == content.Public && p.ID != "not-found"
		if urls[c.Site.BaseURL+p.URL] != public {
			t.Fatalf("sitemap mismatch %s", p.ID)
		}
		page, err := os.ReadFile(filepath.Join(out, "public", content.OutputPath(p.URL)))
		must(err)
		z := html.NewTokenizer(bytes.NewReader(page))
		trackers := 0
		for {
			kind := z.Next()
			if kind == html.ErrorToken {
				break
			}
			token := z.Token()
			if kind == html.StartTagToken && token.Data == "script" {
				attrs := map[string]string{}
				for _, a := range token.Attr {
					attrs[a.Key] = a.Val
				}
				if strings.Contains(attrs["src"], "/assets/stats.") {
					trackers++
					if attrs["data-page-id"] != p.ID || attrs["data-site-timezone"] != c.Site.Timezone {
						t.Fatal("tracker identity", p.ID)
					}
				}
			}
		}
		expected := 0
		if p.StatsEnabled {
			expected = 1
		}
		if trackers != expected {
			t.Fatalf("%s: trackers=%d expected=%d", p.ID, trackers, expected)
		}
	}
	c.Stats.ManifestPath = filepath.Join(out, "meta/manifest.json")
	c.Stats.DatabasePath = filepath.Join(root, "database/stats.db")
	c.Stats.PublicOutput = filepath.Join(root, "stats-public/stats.json")
	c.Stats.PrivateOutput = filepath.Join(root, "stats-private/report.html")
	store, err := stats.OpenWriter(context.Background(), c.Stats.DatabasePath)
	must(err)
	defer store.Close()
	handler, err := stats.NewHandler(c, store, bytes.Repeat([]byte{7}, 32))
	must(err)
	send := func(id, page, referrer string, expected int) {
		t.Helper()
		if len(id) == 1 {
			id = strings.Repeat(id, 32)
		}
		body := fmt.Sprintf(`{"v":1,"event_id":%q,"page_id":%q,"visitor_id":%q,"referrer_host":%q}`, id, page, strings.Repeat("f", 32), referrer)
		r := httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		if response.Code != expected {
			t.Fatalf("event %s: %d %s", page, response.Code, response.Body)
		}
	}
	for n, p := range result.Manifest.Pages {
		if p.StatsEnabled {
			send(fmt.Sprintf("%032x", n+100), p.ID, "search.example", 204)
		}
	}
	send("a", "post:release-notes", "search.example", 204)
	must(stats.GenerateReport(context.Background(), c))
	data, err = os.ReadFile(c.Stats.PublicOutput)
	must(err)
	if strings.Contains(string(data), "review-copy") || strings.Contains(string(data), "lab") {
		t.Fatal("unlisted public report")
	}
	var before stats.PublicReport
	must(json.Unmarshal(data, &before))
	views := before.Pages["post:release-notes"].Views
	source := filepath.Join(c.Paths.Source, "posts/publishing.md")
	text, err := os.ReadFile(source)
	must(err)
	must(os.WriteFile(source, bytes.Replace(text, []byte("slug: publishing-notes"), []byte("slug: renamed-notes"), 1), 0644))
	// Taxonomy display names can change without changing their stable identities.
	must(os.WriteFile(c.Paths.Taxonomies, bytes.Replace(tax, []byte("ESP32 ⚡"), []byte("控制器 · 新名称"), 1), 0644))
	must(os.Remove(filepath.Join(c.Paths.Source, "posts/first-steps.md")))
	must(os.WriteFile(filepath.Join(c.Paths.Source, "posts/new.md"), []byte("---\ntitle: New\nstatus: public\npublished: 2026-10-23\n---\nNew content."), 0644))
	buildConfig := *c
	buildConfig.Stats.ManifestPath = ""
	again, err := build.Build(&buildConfig, out, false)
	must(err)
	if again.Manifest.BuildID == result.Manifest.BuildID {
		t.Fatal("unchanged build identity")
	}
	found := false
	for _, p := range again.Manifest.Pages {
		if p.ID == "post:release-notes" {
			found = true
			if p.URL != "/posts/renamed-notes/" {
				t.Fatal("renamed URL")
			}
		}
		if p.ID == "post:first-steps" {
			t.Fatal("removed ID retained")
		}
	}
	if !found {
		t.Fatal("stable ID lost")
	}
	if _, err = os.Stat(filepath.Join(out, "public/posts/publishing-notes")); !os.IsNotExist(err) {
		t.Fatal("old URL retained")
	}
	send("0", "post:first-steps", "", 400)
	send("1", "post:release-notes", "blog.example", 204)
	must(stats.GenerateReport(context.Background(), c))
	data, err = os.ReadFile(c.Stats.PublicOutput)
	must(err)
	var after stats.PublicReport
	must(json.Unmarshal(data, &after))
	if after.Pages["post:release-notes"].Views != views+1 {
		t.Fatal("stable history lost")
	}
	if _, ok := after.Pages["post:new"]; !ok {
		t.Fatal("new page missing")
	}
	private, err := os.ReadFile(c.Stats.PrivateOutput)
	must(err)
	if !strings.Contains(string(private), "search.example") || !strings.Contains(string(private), "internal") {
		t.Fatal("navigation sources missing")
	}
}
