package stats

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpgradeV1PreservesAggregates(t *testing.T) {
	ctx := context.Background()
	s, path := openStore(t)
	event := visit("old", "post:a", "2026-10-23", "search.example")
	if _, err := s.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("DROP TABLE IF EXISTS event_details; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	upgraded, err := OpenWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var version, pv, uv int
	if err = upgraded.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 2 {
		t.Fatal(version, err)
	}
	if duplicate, err := upgraded.Record(ctx, event); err != nil || !duplicate {
		t.Fatal("dedup lost", duplicate, err)
	}
	if err = upgraded.db.QueryRow("SELECT pv,uv FROM daily_site_stats").Scan(&pv, &uv); err != nil || pv != 1 || uv != 1 {
		t.Fatal("counts changed", pv, uv, err)
	}
	keyPath := filepath.Join(filepath.Dir(path), "visitor.key")
	first, err := LoadOrCreateKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateKey(keyPath)
	if err != nil || string(first) != string(second) {
		t.Fatal("key changed")
	}
}

func TestDetailsTransactionalToggle(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	now := time.Now()
	event := visit("detail", "post:a", now.Format("2006-01-02"), "direct")
	event.Details = &DetailEvent{RecordedAt: now, URL: "/posts/a/", Title: "A", IP: "192.0.2.1", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/140.0 Safari/537.36", Language: "zh-CN"}
	for i := 0; i < 2; i++ {
		if _, err := s.Record(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	event.EventID = "off"
	event.Details = nil
	if _, err := s.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	events, count, err := s.Details(ctx)
	if err != nil || count != 1 || len(events) != 1 {
		t.Fatal(count, err)
	}
	if events[0].Browser != "Chrome" || events[0].OS != "Windows" || events[0].Device != "Desktop" {
		t.Fatal(events[0])
	}
	if _, err = s.db.Exec("CREATE TRIGGER reject_detail BEFORE INSERT ON event_details BEGIN SELECT RAISE(ABORT,'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	event.EventID = "failed"
	event.Details = &DetailEvent{RecordedAt: now, Title: "fail"}
	if _, err = s.Record(ctx, event); err == nil {
		t.Fatal("expected details failure")
	}
	var pv int
	if err = s.db.QueryRow("SELECT pv FROM daily_site_stats").Scan(&pv); err != nil || pv != 2 {
		t.Fatal("partial count", pv, err)
	}
	var seen int
	if err = s.db.QueryRow("SELECT count(*) FROM event_seen WHERE event_id='failed'").Scan(&seen); err != nil || seen != 0 {
		t.Fatal("partial dedup", seen, err)
	}
}

func TestDetailsMigrationRollback(t *testing.T) {
	s, path := openStore(t)
	if _, err := s.db.Exec("DROP TABLE event_details;CREATE TABLE event_details(conflict TEXT);PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if opened, err := OpenWriter(context.Background(), path); err == nil {
		opened.Close()
		t.Fatal("invalid migration accepted")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatal("failed migration committed", version, err)
	}
}

func TestDetailsDedupOutlivesShortEventCache(t *testing.T) {
	s, _ := openStore(t)
	now := time.Now()
	event := visit("long-detail", "post:a", now.Format("2006-01-02"), "direct")
	event.ExpiresAt = now.Add(-time.Hour).Unix()
	event.Details = &DetailEvent{RecordedAt: now}
	if _, err := s.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(context.Background(), now, 168*time.Hour, time.UTC); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := s.Record(context.Background(), event); err != nil || !duplicate {
		t.Fatal("retained details no longer deduplicate", duplicate, err)
	}
}

func TestDetailsRetentionAndReportPrivacy(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	now := time.Now()
	old := visit("old-detail", "post:a", now.Format("2006-01-02"), "direct")
	old.Details = &DetailEvent{RecordedAt: now.Add(-169 * time.Hour), IP: "203.0.113.8", Title: "<script>unsafe</script>"}
	if _, err := s.Record(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(ctx, now, 168*time.Hour, time.UTC, 168*time.Hour); err != nil {
		t.Fatal(err)
	}
	_, count, err := s.Details(ctx)
	if err != nil || count != 0 {
		t.Fatal("old detail retained", count, err)
	}
	var pv int
	if err = s.db.QueryRow("SELECT pv FROM daily_site_stats").Scan(&pv); err != nil || pv != 1 {
		t.Fatal("aggregate deleted", pv, err)
	}
	for _, test := range []struct{ ua, browser, os, device string }{
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1", "Safari", "iOS", "Mobile"},
		{"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.7339.123 Mobile Safari/537.36", "Chrome", "Android", "Mobile"},
		{"unexpected-agent", "Unknown", "Unknown", "Unknown"},
	} {
		b, o, d := classifyUA(test.ua)
		if b != test.browser || o != test.os || d != test.device {
			t.Fatal(b, o, d)
		}
	}
	if got := preferredLanguage("en-US;q=0.5, zh-CN;q=1"); got != "zh-CN" {
		t.Fatal(got)
	}
	if got := boundedHeader(strings.Repeat("中", 1000), 512); len(got) > 512 {
		t.Fatal("header unbounded")
	}
}
