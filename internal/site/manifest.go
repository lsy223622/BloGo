package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

type Manifest struct {
	SchemaVersion int          `json:"schema_version"`
	BuildID       string       `json:"build_id"`
	Fingerprint   string       `json:"fingerprint"`
	GeneratedAt   time.Time    `json:"generated_at"`
	Pages         []PageRecord `json:"pages"`
}
type PageRecord struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	URL          string         `json:"url"`
	Title        string         `json:"title"`
	Visibility   content.Status `json:"visibility"`
	StatsEnabled bool           `json:"stats_enabled"`
	Published    *time.Time     `json:"published,omitempty"`
}

var pageIdentity = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*(?::[a-z0-9]+(?:-[a-z0-9]+)*)*$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (m *Manifest) Validate(preview bool) error {
	if m.SchemaVersion != 1 || m.BuildID == "" || !digestPattern.MatchString(m.Fingerprint) || m.GeneratedAt.IsZero() {
		return fmt.Errorf("invalid manifest header")
	}
	ids, urls := map[string]bool{}, map[string]bool{}
	for _, p := range m.Pages {
		if !pageIdentity.MatchString(p.ID) || ids[p.ID] || urls[p.URL] {
			return fmt.Errorf("duplicate or invalid manifest page %s %s", p.ID, p.URL)
		}
		ids[p.ID] = true
		urls[p.URL] = true
		u, err := content.CanonicalURL(p.URL)
		if err != nil || u != p.URL {
			return fmt.Errorf("noncanonical manifest URL %s", p.URL)
		}
		if p.Visibility != content.Public && p.Visibility != content.Unlisted && !(preview && p.Visibility == content.Draft) {
			return fmt.Errorf("invalid manifest visibility %s", p.ID)
		}
		if p.Visibility != content.Draft && strings.TrimSpace(p.Title) == "" {
			return fmt.Errorf("missing manifest title %s", p.ID)
		}
		switch p.Kind {
		case "post", "page":
			if !strings.HasPrefix(p.ID, p.Kind+":") || !content.ValidID(strings.TrimPrefix(p.ID, p.Kind+":")) {
				return fmt.Errorf("invalid content manifest identity %s", p.ID)
			}
			if p.Kind == "post" && p.Visibility != content.Draft && p.Published == nil {
				return fmt.Errorf("post missing published date %s", p.ID)
			}
		case "system", "category", "tag", "archive", "pagination":
			if p.Visibility != content.Public || !p.StatsEnabled {
				return fmt.Errorf("invalid system page visibility/stats %s", p.ID)
			}
		default:
			return fmt.Errorf("invalid manifest kind %s", p.Kind)
		}
	}
	return nil
}

func ReadManifest(file string) (*Manifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var m Manifest
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&m); err != nil {
		return nil, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("extra manifest JSON")
	}
	if err = m.Validate(false); err != nil {
		return nil, err
	}
	return &m, nil
}
