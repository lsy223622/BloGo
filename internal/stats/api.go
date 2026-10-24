package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/requestmeta"
	"github.com/lsy223622/BloGo/internal/site"
)

var browserID = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Event struct {
	Version                                  int
	EventID, PageID, VisitorID, ReferrerHost string
}
type Handler struct {
	config       *config.Config
	store        *Store
	key          []byte
	zone         *time.Location
	origin, host string
	now          func() time.Time
	limit        *rateLimit
	sources      sourceLimits
}
type rateLimit struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (l *rateLimit) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.After(l.last) {
		l.tokens = math.Min(200, l.tokens+now.Sub(l.last).Seconds()*100)
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

func NewHandler(c *config.Config, store *Store, key []byte) (*Handler, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("expected 32-byte visitor key")
	}
	if _, err := site.ReadManifest(c.Stats.ManifestPath); err != nil {
		return nil, err
	}
	origin, err := normalizedOrigin(c.Site.BaseURL)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(c.Site.BaseURL)
	zone, err := time.LoadLocation(c.Site.Timezone)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &Handler{config: c, store: store, key: append([]byte(nil), key...), zone: zone, origin: origin, host: base.Hostname(), now: time.Now, limit: &rateLimit{tokens: 200, last: now}}, nil
}

func decodeEvent(body []byte) (Event, error) {
	if !utf8.Valid(body) {
		return Event{}, fmt.Errorf("invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return Event{}, fmt.Errorf("expected object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return Event{}, err
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return Event{}, fmt.Errorf("duplicate field")
		}
		switch name {
		case "v", "event_id", "page_id", "visitor_id", "referrer_host":
		default:
			return Event{}, fmt.Errorf("unknown field")
		}
		seen[name] = true
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return Event{}, err
		}
	}
	if _, err = decoder.Token(); err != nil {
		return Event{}, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return Event{}, fmt.Errorf("extra JSON value")
	}
	if len(seen) != 5 {
		return Event{}, fmt.Errorf("missing field")
	}
	var wire struct {
		Version      *int    `json:"v"`
		EventID      *string `json:"event_id"`
		PageID       *string `json:"page_id"`
		VisitorID    *string `json:"visitor_id"`
		ReferrerHost *string `json:"referrer_host"`
	}
	if err = json.Unmarshal(body, &wire); err != nil {
		return Event{}, err
	}
	if wire.Version == nil || wire.EventID == nil || wire.PageID == nil || wire.VisitorID == nil || wire.ReferrerHost == nil {
		return Event{}, fmt.Errorf("null field")
	}
	e := Event{Version: *wire.Version, EventID: *wire.EventID, PageID: *wire.PageID, VisitorID: *wire.VisitorID, ReferrerHost: *wire.ReferrerHost}
	if e.Version != 1 || !browserID.MatchString(e.EventID) || !browserID.MatchString(e.VisitorID) {
		return Event{}, fmt.Errorf("invalid version or browser identity")
	}
	return e, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path != "/api/events" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "unsupported media type", 415)
		return
	}
	now := h.now()
	ip := requestmeta.ClientIP(r)
	if !ip.IsValid() {
		http.Error(w, "invalid peer", 400)
		return
	}
	if !h.limit.allow(now) || !h.sources.allow(ip.String(), now) {
		http.Error(w, "rate limit exceeded", 429)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			http.Error(w, "request too large", 413)
		} else {
			http.Error(w, "invalid event", 400)
		}
		return
	}
	event, err := decodeEvent(body)
	if err != nil {
		http.Error(w, "invalid event", 400)
		return
	}
	manifest, err := site.ReadManifest(h.config.Stats.ManifestPath)
	if err != nil {
		http.Error(w, "statistics unavailable", 503)
		return
	}
	var current *site.PageRecord
	for n := range manifest.Pages {
		p := &manifest.Pages[n]
		if p.ID == event.PageID {
			current = p
			break
		}
	}
	if current == nil || !current.StatsEnabled || (current.Visibility != content.Public && current.Visibility != content.Unlisted) {
		http.Error(w, "invalid event", 400)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		normalized, err := normalizedOrigin(origin)
		if err != nil || normalized != h.origin {
			http.Error(w, "invalid event context", 400)
			return
		}
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		ref, err := url.Parse(referer)
		if err != nil || ref.User != nil {
			http.Error(w, "invalid event context", 400)
			return
		}
		normalized, err := normalizedOrigin(ref.Scheme + "://" + ref.Host)
		if err != nil || normalized != h.origin {
			http.Error(w, "invalid event context", 400)
			return
		}
		// An old open document can retain its previous URL after publication. Only
		// reject a path currently assigned to a different known page; 404 paths
		// and old URLs never become persisted event fields.
		for _, p := range manifest.Pages {
			if p.URL == ref.Path && p.ID != event.PageID {
				http.Error(w, "invalid event context", 400)
				return
			}
		}
	}
	source, err := NormalizeSource(event.ReferrerHost, h.host)
	if err != nil {
		http.Error(w, "invalid event", 400)
		return
	}
	day := now.In(h.zone).Format("2006-01-02")
	record := Record{EventID: event.EventID, PageID: event.PageID, Day: day, Source: source, VisitorHash: VisitorDigest(h.key, day, event.VisitorID), ExpiresAt: now.Add(time.Duration(h.config.Stats.EventRetention)).Unix()}
	if h.config.Stats.Details.Enabled {
		record.Details = &DetailEvent{RecordedAt: now, URL: current.URL, Title: current.Title, IP: ip.String(), UserAgent: boundedHeader(r.UserAgent(), 512), Language: preferredLanguage(r.Header.Get("Accept-Language"))}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err = h.store.Record(ctx, record); err != nil {
		http.Error(w, "statistics unavailable", 503)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
