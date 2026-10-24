package stats

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	useragent "github.com/medama-io/go-useragent"
	"golang.org/x/text/language"
)

const detailsSchema = `CREATE TABLE event_details(
 event_id TEXT PRIMARY KEY,recorded_at INTEGER NOT NULL,page_id TEXT NOT NULL,
 url TEXT NOT NULL,title TEXT NOT NULL,ip TEXT NOT NULL,user_agent TEXT NOT NULL,
 language TEXT NOT NULL,browser TEXT NOT NULL,os TEXT NOT NULL,device TEXT NOT NULL,
 visitor_hash BLOB NOT NULL CHECK(length(visitor_hash)=32),source TEXT NOT NULL);
 CREATE INDEX idx_event_details_time ON event_details(recorded_at);
 PRAGMA user_version=2;`

type DetailEvent struct {
	RecordedAt                                                                                       time.Time
	EventID, PageID, URL, Title, IP, UserAgent, Language, Browser, OS, Device, VisitorDigest, Source string
}

var agentParser = useragent.NewParser()

func classifyUA(value string) (string, string, string) {
	parsed := agentParser.Parse(value)
	known := func(value string) string {
		if value == "" {
			return "Unknown"
		}
		return value
	}
	return known(parsed.Browser().String()), known(parsed.OS().String()), known(parsed.Device().String())
}

func boundedHeader(value string, max int) string {
	value = strings.ToValidUTF8(value, "")
	if len(value) > max {
		value = value[:max]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func preferredLanguage(value string) string {
	tags, _, err := language.ParseAcceptLanguage(boundedHeader(value, 256))
	if err != nil || len(tags) == 0 {
		return "Unknown"
	}
	return tags[0].String()
}

func readDetails(ctx context.Context, tx *sql.Tx) ([]DetailEvent, int, error) {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM event_details").Scan(&count); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT recorded_at,event_id,page_id,url,title,ip,user_agent,language,browser,os,device,lower(hex(visitor_hash)),source FROM event_details ORDER BY recorded_at DESC,event_id LIMIT 1000`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []DetailEvent{}
	for rows.Next() {
		var detail DetailEvent
		var recorded int64
		if err = rows.Scan(&recorded, &detail.EventID, &detail.PageID, &detail.URL, &detail.Title, &detail.IP, &detail.UserAgent, &detail.Language, &detail.Browser, &detail.OS, &detail.Device, &detail.VisitorDigest, &detail.Source); err != nil {
			return nil, 0, err
		}
		detail.RecordedAt = time.UnixMilli(recorded)
		result = append(result, detail)
	}
	return result, count, rows.Err()
}

func (s *Store) Details(ctx context.Context) ([]DetailEvent, int, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	events, count, err := readDetails(ctx, tx)
	if err != nil {
		return nil, 0, err
	}
	return events, count, tx.Commit()
}
