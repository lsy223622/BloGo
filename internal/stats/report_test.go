package stats

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/filelock"
)

func TestReportVisibilityHistoryAndReadOnlySnapshot(t *testing.T) {
	_, s, c, m := apiFixture(t)
	c.Stats.DatabasePath = databasePath(t, s)
	c.Stats.PublicOutput = filepath.Join(t.TempDir(), "public/stats.json")
	c.Stats.PrivateOutput = filepath.Join(t.TempDir(), "private/report.html")
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	for n, page := range []string{"post:a", "page:secret", "home", "page:deleted"} {
		_, err := s.Record(context.Background(), Record{EventID: page, PageID: page, Day: "2026-10-23", Source: "search.example", VisitorHash: VisitorDigest([]byte("key"), "day", string(rune('a'+n))), ExpiresAt: now.Add(time.Hour).Unix()})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := generateReport(context.Background(), c, now, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(c.Stats.PublicOutput)
	if err != nil {
		t.Fatal(err)
	}
	var report PublicReport
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.BuildID != m.BuildID || report.Pages["post:a"].Views != 1 || report.Pages["home"].Views != 1 || len(report.Pages) != 3 || len(report.Popular30D) != 1 {
		t.Fatalf("report: %s", data)
	}
	for _, secret := range []string{"secret", "deleted", "disabled"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("public leak", secret)
		}
	}
	private, err := os.ReadFile(c.Stats.PrivateOutput)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Secret", "page:deleted", "已删除", "search.example", "7 天", "30 天", "UV"} {
		if !strings.Contains(string(private), want) {
			t.Fatal("private report missing", want)
		}
	}
	m.Pages[1].Visibility = content.Unlisted
	saveManifest(t, c.Stats.ManifestPath, m)
	if err = generateReport(context.Background(), c, now, nil); err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(c.Stats.PublicOutput)
	if strings.Contains(string(changed), "post:a") {
		t.Fatal("visibility did not refresh")
	}
	var pv int
	if err = s.db.QueryRow("SELECT sum(pv) FROM daily_page_stats").Scan(&pv); err != nil || pv != 4 {
		t.Fatalf("report modified business data: %d %v", pv, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = generateReport(context.Background(), c, now, nil); err != nil {
		t.Fatal("writer stopped report", err)
	}
}

func databasePath(t *testing.T, s *Store) string {
	t.Helper()
	rows, err := s.db.Query("PRAGMA database_list")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var seq int
	var name, path string
	if !rows.Next() {
		t.Fatal("missing db")
	}
	if err = rows.Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReportLocksAndManifestChangePreserveOutputs(t *testing.T) {
	_, s, c, m := apiFixture(t)
	c.Stats.DatabasePath = databasePath(t, s)
	c.Stats.PublicOutput = filepath.Join(t.TempDir(), "stats.json")
	c.Stats.PrivateOutput = filepath.Join(t.TempDir(), "report.html")
	for _, path := range []string{c.Stats.PublicOutput, c.Stats.PrivateOutput} {
		if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	lock, err := filelock.Acquire(c.Stats.PrivateOutput + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if err = generateReport(context.Background(), c, time.Now(), nil); err == nil {
		t.Fatal("competing report accepted")
	}
	lock.Close()
	err = generateReport(context.Background(), c, time.Now(), func() { m.BuildID = "next"; saveManifest(t, c.Stats.ManifestPath, m) })
	if err == nil {
		t.Fatal("stale report published")
	}
	for _, path := range []string{c.Stats.PublicOutput, c.Stats.PrivateOutput} {
		data, _ := os.ReadFile(path)
		if string(data) != "previous" {
			t.Fatal("old output changed")
		}
	}
	if err = os.WriteFile(c.Stats.ManifestPath, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = generateReport(context.Background(), c, time.Now(), nil); err == nil {
		t.Fatal("invalid manifest accepted")
	}
}
