package stats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func visit(id, page, day, source string) Record {
	return Record{EventID: id, PageID: page, Day: day, Source: source, VisitorHash: VisitorDigest([]byte("01234567890123456789012345678901"), day, "visitor"), ExpiresAt: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC).Unix()}
}
func openStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.db")
	store, err := OpenWriter(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store, path
}

func TestTransactionsDedupVisitorsAndSources(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	event := visit("event-a", "post:a", "2026-10-23", "search.example")
	if duplicate, err := s.Record(ctx, event); err != nil || duplicate {
		t.Fatalf("record: %v %v", duplicate, err)
	}
	if duplicate, err := s.Record(ctx, event); err != nil || !duplicate {
		t.Fatal("event not deduplicated")
	}
	if _, err := s.Record(ctx, visit("event-b", "post:b", "2026-10-23", "search.example")); err != nil {
		t.Fatal(err)
	}
	var pv, uv int
	if err := s.db.QueryRow("SELECT pv,uv FROM daily_site_stats WHERE day='2026-10-23'").Scan(&pv, &uv); err != nil || pv != 2 || uv != 1 {
		t.Fatalf("counts %d %d %v", pv, uv, err)
	}
	var sources int
	if err := s.db.QueryRow("SELECT count(*) FROM daily_referrers WHERE source='search.example'").Scan(&sources); err != nil || sources != 2 {
		t.Fatal("page sources merged")
	}
	if _, err := s.Record(ctx, visit("event-c", "post:a", "2026-10-24", "direct")); err != nil {
		t.Fatal(err)
	}
	var visits int
	if err := s.db.QueryRow("SELECT count(*) FROM visitor_seen").Scan(&visits); err != nil || visits != 2 {
		t.Fatal("cross-day identity reused")
	}
}

func TestTransactionFailureRollsBackEverything(t *testing.T) {
	s, _ := openStore(t)
	if _, err := s.db.Exec("CREATE TRIGGER reject_referrer BEFORE INSERT ON daily_referrers BEGIN SELECT RAISE(ABORT,'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Record(context.Background(), visit("event-a", "post:a", "2026-10-23", "direct")); err == nil {
		t.Fatal("expected transaction failure")
	}
	for _, table := range []string{"event_seen", "visitor_seen", "daily_site_stats", "daily_page_stats", "daily_referrers"} {
		var count int
		if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial write %s: %d %v", table, count, err)
		}
	}
}

func TestConcurrentDuplicateAndReadOnlyLifecycle(t *testing.T) {
	s, path := openStore(t)
	var wg sync.WaitGroup
	failures := make(chan error, 24)
	for n := 0; n < 24; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Record(context.Background(), visit("same", "post:a", "2026-10-23", "internal"))
			if err != nil {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	reader, err := OpenReader(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	var pv int
	if err = reader.db.QueryRow("SELECT pv FROM daily_site_stats").Scan(&pv); err != nil || pv != 1 {
		t.Fatalf("concurrent count %d: %v", pv, err)
	}
	if _, err = reader.db.Exec("UPDATE daily_site_stats SET pv=99"); err == nil {
		t.Fatal("report connection wrote business data")
	}
	reader.Close()
	s.Close()
	reader, err = OpenReader(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err = reader.db.QueryRow("SELECT pv FROM daily_site_stats").Scan(&pv); err != nil || pv != 1 {
		t.Fatal("reader failed after writer stop")
	}
	reader.Close()
	restarted, err := OpenWriter(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	reader, err = OpenReader(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err = reader.db.QueryRow("SELECT pv FROM daily_site_stats").Scan(&pv); err != nil || pv != 1 {
		t.Fatal("reader failed after writer restart")
	}
}

func TestCleanupPreservesAggregatesAndKey(t *testing.T) {
	s, _ := openStore(t)
	if _, err := s.Record(context.Background(), visit("old", "post:a", "2026-10-23", "direct")); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(context.Background(), time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), 7*24*time.Hour, time.UTC); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		table string
		want  int
	}{{"event_seen", 0}, {"visitor_seen", 0}, {"daily_site_stats", 1}, {"daily_page_stats", 1}, {"daily_referrers", 1}} {
		var count int
		if err := s.db.QueryRow("SELECT count(*) FROM " + tc.table).Scan(&count); err != nil || count != tc.want {
			t.Fatalf("cleanup %s %d: %v", tc.table, count, err)
		}
	}
	path := filepath.Join(t.TempDir(), "visitor.key")
	a, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateKey(path)
	if err != nil || string(a) != string(b) || len(a) != 32 {
		t.Fatal("key not persistent")
	}
}

func TestWALReadSnapshotDoesNotBlockCollection(t *testing.T) {
	s, path := openStore(t)
	ctx := context.Background()
	if _, err := s.Record(ctx, visit("a", "post:a", "2026-10-23", "direct")); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReader(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var old int
	if err = tx.QueryRow("SELECT pv FROM daily_site_stats").Scan(&old); err != nil {
		t.Fatal(err)
	}
	writeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if _, err = s.Record(writeCtx, visit("b", "post:b", "2026-10-23", "direct")); err != nil {
		t.Fatalf("reader blocked writer: %v", err)
	}
	var snapshot int
	if err = tx.QueryRow("SELECT pv FROM daily_site_stats").Scan(&snapshot); err != nil || snapshot != old {
		t.Fatal("read transaction is not consistent")
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var current int
	if err = reader.db.QueryRow("SELECT pv FROM daily_site_stats").Scan(&current); err != nil || current != 2 {
		t.Fatal("new report did not see committed data")
	}
}

func TestLockDeadlineDoesNotPartiallyWrite(t *testing.T) {
	s, path := openStore(t)
	other, err := OpenWriter(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var timeout int
	var mode, version string
	if err = s.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatalf("busy timeout %d %v", timeout, err)
	}
	if err = s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal mode %s %v", mode, err)
	}
	if err = s.db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Log("SQLite", version)
	lock, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err = other.Record(ctx, visit("blocked", "post:a", "2026-10-23", "direct")); err == nil {
		t.Fatal("locked write succeeded")
	}
	t.Log("canceled lock wait", time.Since(start))
	lock.Rollback()
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM event_seen").Scan(&count); err != nil || count != 0 {
		t.Fatal("timed-out record partially written")
	}
}

func TestUnknownSchemaAndInvalidKeyAreNotReinitialized(t *testing.T) {
	for _, version := range []int{0, 2} {
		s, path := openStore(t)
		if _, err := s.db.Exec("PRAGMA user_version=" + fmt.Sprint(version)); err != nil {
			t.Fatal(err)
		}
		s.Close()
		if other, err := OpenWriter(context.Background(), path); err == nil {
			other.Close()
			t.Fatalf("accepted schema version %d", version)
		}
	}
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path); err == nil {
		t.Fatal("invalid key replaced")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "invalid" {
		t.Fatal("invalid key was changed")
	}
}
