package stats

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVacuumBackupAndKeyRestoreContinueCollection(t *testing.T) {
	ctx := context.Background()
	_, s, c, _ := apiFixture(t)
	keyPath := filepath.Join(t.TempDir(), "visitor.key")
	key, err := LoadOrCreateKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	day := "2026-10-23"
	event := Record{EventID: "before", PageID: "post:a", Day: day, Source: "search.example", VisitorHash: VisitorDigest(key, day, "browser"), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	if _, err = s.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if _, err = s.db.ExecContext(ctx, "VACUUM INTO ?", backup); err != nil {
		t.Fatal(err)
	}
	restoredKey := filepath.Join(t.TempDir(), "visitor.key")
	if err = os.WriteFile(restoredKey, key, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenWriter(ctx, backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	loaded, err := LoadOrCreateKey(restoredKey)
	if err != nil {
		t.Fatal(err)
	}
	event.VisitorHash = VisitorDigest(loaded, day, "browser")
	if duplicate, err := restored.Record(ctx, event); err != nil || !duplicate {
		t.Fatal("dedup lost after restore", err)
	}
	event.EventID = "after"
	if _, err = restored.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	var pv, uv int
	if err = restored.db.QueryRow("SELECT pv,uv FROM daily_site_stats").Scan(&pv, &uv); err != nil || pv != 2 || uv != 1 {
		t.Fatalf("restored counts %d %d %v", pv, uv, err)
	}
	var integrity string
	if err = restored.db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("backup integrity", err)
	}
	c.Stats.DatabasePath = backup
	c.Stats.PublicOutput = filepath.Join(t.TempDir(), "stats.json")
	c.Stats.PrivateOutput = filepath.Join(t.TempDir(), "report.html")
	if err = GenerateReport(ctx, c); err != nil {
		t.Fatal("restored report", err)
	}
}
