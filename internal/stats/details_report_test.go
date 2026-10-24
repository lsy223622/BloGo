package stats

import (
	"context"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/requestmeta"
)

func TestDetailedReportPrivacy(t *testing.T) {
	h, s, c, m := apiFixture(t)
	c.Stats.Details.Enabled = true
	c.Stats.DatabasePath = databasePath(t, s)
	c.Stats.PublicOutput = filepath.Join(t.TempDir(), "public/stats.json")
	c.Stats.PrivateOutput = filepath.Join(t.TempDir(), "private/report.html")
	m.Pages[2].Title = "<script>Private</script>"
	saveManifest(t, c.Stats.ManifestPath, m)
	r := httptest.NewRequest("POST", "/api/events", strings.NewReader(eventBody("a", "page:secret", "search.example")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "<img src=x onerror=alert(1)>")
	r.Header.Set("Accept-Language", "zh-CN,en;q=0.8")
	r = requestmeta.WithClientIP(r, netip.MustParseAddr("203.0.113.7"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if err := generateReport(context.Background(), c, h.now(), nil); err != nil {
		t.Fatal(err)
	}
	public, err := os.ReadFile(c.Stats.PublicOutput)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"203.0.113.7", "page:secret", "user_agent", "visitor_hash", "zh-CN"} {
		if strings.Contains(string(public), private) {
			t.Fatal("public detail leak", private)
		}
	}
	report, err := os.ReadFile(c.Stats.PrivateOutput)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"203.0.113.7", "zh-CN", "&lt;script&gt;Private&lt;/script&gt;", "&lt;img src=x onerror=alert(1)&gt;", "最近事件"} {
		if !strings.Contains(string(report), want) {
			t.Fatal("private detail missing", want)
		}
	}
	if strings.Contains(string(report), "<script>Private") || strings.Contains(string(report), "<img src=x") {
		t.Fatal("unescaped details")
	}
}

func TestDetailedRowsBoundAndShortenedRetention(t *testing.T) {
	s, _ := openStore(t)
	now := time.Now()
	ctx := context.Background()
	_, err := s.db.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1005) INSERT INTO event_details SELECT 'row-'||x,?+x,'post:a','/posts/a/','A','192.0.2.1','UA','zh-CN','Unknown','Unknown','Unknown',zeroblob(32),'direct' FROM n`, now.Add(-2*time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	rows, count, err := s.Details(ctx)
	if err != nil || count != 1005 || len(rows) != 1000 {
		t.Fatal(len(rows), count, err)
	}
	if err = s.Cleanup(ctx, now, 168*time.Hour, time.UTC, 168*time.Hour); err != nil {
		t.Fatal(err)
	}
	_, count, err = s.Details(ctx)
	if err != nil || count != 1005 {
		t.Fatal("premature cleanup", count, err)
	}
	if err = s.Cleanup(ctx, now, 168*time.Hour, time.UTC, time.Hour); err != nil {
		t.Fatal(err)
	}
	_, count, err = s.Details(ctx)
	if err != nil || count != 0 {
		t.Fatal("shortened retention not applied", count, err)
	}
}

func TestHandlerDetailsToggle(t *testing.T) {
	h, s, c, _ := apiFixture(t)
	for _, step := range []struct {
		id      string
		enabled bool
	}{{"a", true}, {"b", false}, {"c", true}, {"b", true}} {
		c.Stats.Details.Enabled = step.enabled
		if response := sendEvent(h, eventBody(step.id, "post:a", ""), "", ""); response.Code != 204 {
			t.Fatal(response.Code, response.Body)
		}
	}
	rows, count, err := s.Details(context.Background())
	if err != nil || count != 2 {
		t.Fatal(count, err)
	}
	for _, row := range rows {
		if row.EventID == strings.Repeat("b", 32) {
			t.Fatal("disabled event backfilled")
		}
	}
	var pv, uv int
	if err = s.db.QueryRow("SELECT pv,uv FROM daily_site_stats").Scan(&pv, &uv); err != nil || pv != 3 || uv != 1 {
		t.Fatal(pv, uv, err)
	}
}
