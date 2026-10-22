package stats

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db       *sql.DB
	writable bool
}
type Record struct {
	EventID, PageID, Day, Source string
	VisitorHash                  []byte
	ExpiresAt                    int64
}

func OpenWriter(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0660)
	if err == nil {
		if err = file.Close(); err != nil {
			return nil, err
		}
	} else if !os.IsExist(err) {
		return nil, err
	}
	return open(ctx, path, true)
}
func OpenReader(ctx context.Context, path string) (*Store, error) { return open(ctx, path, false) }
func open(ctx context.Context, path string, write bool) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	filePath := filepath.ToSlash(abs)
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}
	u := url.URL{Scheme: "file", Path: filePath}
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	if write {
		query.Set("mode", "rwc")
		query.Add("_pragma", "journal_mode(WAL)")
		query.Set("_txlock", "immediate")
	} else {
		query.Set("mode", "ro")
		query.Add("_pragma", "query_only(1)")
	}
	u.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, writable: write}
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if write {
		err = s.initialize(ctx)
	} else {
		var version int
		err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
		if err == nil && version != 1 {
			err = fmt.Errorf("unsupported statistics schema version %d", version)
		}
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

const schema = `
CREATE TABLE daily_site_stats(day TEXT PRIMARY KEY,pv INTEGER NOT NULL DEFAULT 0 CHECK(pv>=0),uv INTEGER NOT NULL DEFAULT 0 CHECK(uv>=0));
CREATE TABLE daily_page_stats(day TEXT NOT NULL,page_id TEXT NOT NULL,pv INTEGER NOT NULL DEFAULT 0 CHECK(pv>=0),PRIMARY KEY(day,page_id));
CREATE INDEX idx_daily_page_stats_page_day ON daily_page_stats(page_id,day);
CREATE TABLE daily_referrers(day TEXT NOT NULL,page_id TEXT NOT NULL,source TEXT NOT NULL,pv INTEGER NOT NULL DEFAULT 0 CHECK(pv>=0),PRIMARY KEY(day,page_id,source));
CREATE TABLE visitor_seen(day TEXT NOT NULL,visitor_hash BLOB NOT NULL CHECK(length(visitor_hash)=32),PRIMARY KEY(day,visitor_hash));
CREATE TABLE event_seen(event_id TEXT PRIMARY KEY,expires_at INTEGER NOT NULL);
CREATE INDEX idx_event_seen_expires ON event_seen(expires_at);
PRAGMA user_version=1;`

func (s *Store) initialize(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version == 0 {
		var tables int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
			return err
		}
		if tables != 0 {
			return fmt.Errorf("refusing to initialize nonempty unversioned database")
		}
		if _, err = tx.ExecContext(ctx, schema); err != nil {
			return err
		}
	} else if version != 1 {
		return fmt.Errorf("unsupported statistics schema version %d", version)
	}
	for _, query := range []string{"SELECT day,pv,uv FROM daily_site_stats LIMIT 0", "SELECT day,page_id,pv FROM daily_page_stats LIMIT 0", "SELECT day,page_id,source,pv FROM daily_referrers LIMIT 0", "SELECT day,visitor_hash FROM visitor_seen LIMIT 0", "SELECT event_id,expires_at FROM event_seen LIMIT 0"} {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		if err = rows.Close(); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Record(ctx context.Context, event Record) (bool, error) {
	if !s.writable {
		return false, fmt.Errorf("read-only statistics connection")
	}
	if _, err := time.Parse("2006-01-02", event.Day); err != nil {
		return false, err
	}
	if event.EventID == "" || event.PageID == "" || event.Source == "" || len(event.VisitorHash) != 32 || event.ExpiresAt <= 0 {
		return false, fmt.Errorf("invalid internal event record")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	inserted, err := tx.ExecContext(ctx, "INSERT INTO event_seen(event_id,expires_at) VALUES(?,?) ON CONFLICT(event_id) DO NOTHING", event.EventID, event.ExpiresAt)
	if err != nil {
		return false, err
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, tx.Commit()
	}
	visitor, err := tx.ExecContext(ctx, "INSERT INTO visitor_seen(day,visitor_hash) VALUES(?,?) ON CONFLICT(day,visitor_hash) DO NOTHING", event.Day, event.VisitorHash)
	if err != nil {
		return false, err
	}
	uv, err := visitor.RowsAffected()
	if err != nil {
		return false, err
	}
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO daily_site_stats(day,pv,uv) VALUES(?,1,?) ON CONFLICT(day) DO UPDATE SET pv=pv+1,uv=uv+excluded.uv", []any{event.Day, uv}},
		{"INSERT INTO daily_page_stats(day,page_id,pv) VALUES(?,?,1) ON CONFLICT(day,page_id) DO UPDATE SET pv=pv+1", []any{event.Day, event.PageID}},
		{"INSERT INTO daily_referrers(day,page_id,source,pv) VALUES(?,?,?,1) ON CONFLICT(day,page_id,source) DO UPDATE SET pv=pv+1", []any{event.Day, event.PageID, event.Source}},
	} {
		if _, err = tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return false, err
		}
	}
	return false, tx.Commit()
}

func (s *Store) Cleanup(ctx context.Context, now time.Time, visitorRetention time.Duration, zone *time.Location) error {
	if !s.writable {
		return fmt.Errorf("read-only statistics connection")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM event_seen WHERE expires_at<=?", now.Unix()); err != nil {
		return err
	}
	cutoff := now.Add(-visitorRetention).In(zone).Format("2006-01-02")
	if _, err = tx.ExecContext(ctx, "DELETE FROM visitor_seen WHERE day<?", cutoff); err != nil {
		return err
	}
	return tx.Commit()
}
