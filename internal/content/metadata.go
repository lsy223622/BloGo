package content

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/lsy223622/BloGo/internal/yamlutil"
)

type Status string

const (
	Public   Status = "public"
	Unlisted Status = "unlisted"
	Draft    Status = "draft"
)

func (item *ContentItem) ProductionVisible() bool {
	return item.Status == Public || item.Status == Unlisted
}
func (item *ContentItem) PubliclyListed() bool { return item.Status == Public }
func (item *ContentItem) NoIndex() bool        { return item.Status == Unlisted }

// Validate resolves all content metadata using only the supplied site timezone
// and taxonomy declarations. File timestamps never participate in the model.
func (item *ContentItem) Validate(location *time.Location, taxonomies *Taxonomies) error {
	if location == nil || taxonomies == nil {
		return fmt.Errorf("%s: timezone and taxonomies are required", item.SourcePath)
	}
	if err := item.ResolveIdentity(); err != nil {
		return err
	}
	item.Status = Status(item.Meta.Status)
	if yamlutil.Field(item.node, "status") == item.node {
		item.Status = Draft
	}
	switch item.Status {
	case Public, Unlisted, Draft:
	default:
		return item.fieldError("status", "expected public, unlisted or draft")
	}
	if item.Status != Draft && strings.TrimSpace(item.Meta.Title) == "" {
		return item.fieldError("title", "required for published content")
	}
	item.Layout = item.Meta.Layout
	if yamlutil.Field(item.node, "layout") == item.node {
		if item.Kind == Post {
			item.Layout = "article"
		} else if item.Format == HTML {
			item.Layout = "standalone"
		} else {
			item.Layout = "page"
		}
	}
	valid := false
	if item.Kind == Post {
		valid = item.Format == Markdown && item.Layout == "article"
	} else if item.Format == Markdown {
		valid = item.Layout == "page" || item.Layout == "article"
	} else {
		valid = item.Layout == "page" || item.Layout == "standalone"
	}
	if !valid {
		return item.fieldError("layout", "invalid layout for this source type")
	}
	var err error
	item.Published, err = parseDate(item.Meta.Published, location)
	if err != nil {
		return item.fieldError("published", err.Error())
	}
	item.Updated, err = parseDate(item.Meta.Updated, location)
	if err != nil {
		return item.fieldError("updated", err.Error())
	}
	if item.Kind == Post && item.Status != Draft && item.Published == nil {
		return item.fieldError("published", "required for Public and Unlisted posts")
	}
	if item.Updated == nil && item.Published != nil {
		v := *item.Published
		item.Updated = &v
	}
	if item.Published != nil && item.Updated != nil && item.Updated.Before(*item.Published) {
		return item.fieldError("updated", "must not precede published")
	}
	item.Stats = true
	if item.Meta.Stats != nil {
		item.Stats = *item.Meta.Stats
	}
	var categories, tags []string
	seen := map[string]bool{}
	for _, id := range item.Meta.Categories {
		if !ValidID(id) {
			return item.fieldError("categories", "invalid ASCII stable ID "+id)
		}
		if _, ok := taxonomies.Categories[id]; !ok {
			return item.fieldError("categories", "unknown category "+id)
		}
		if !seen[id] {
			categories = append(categories, id)
			seen[id] = true
		}
	}
	seen = map[string]bool{}
	for _, id := range item.Meta.Tags {
		if !ValidID(id) {
			return item.fieldError("tags", "invalid ASCII stable ID "+id)
		}
		if _, ok := taxonomies.Tags[id]; !ok {
			return item.fieldError("tags", "unknown tag "+id)
		}
		if !seen[id] {
			tags = append(tags, id)
			seen[id] = true
		}
	}
	item.Meta.Categories = categories
	item.Meta.Tags = tags
	return nil
}

func parseDate(value *DateText, location *time.Location) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	raw := string(*value)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z0700", "2006-01-02T15:04Z07:00", "2006-01-02T15:04Z0700"} {
		if t, err := time.Parse(layout, raw); err == nil {
			t = t.In(location)
			return &t, nil
		}
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05.999999999", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, raw, location); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("expected valid ISO 8601 / RFC 3339 timestamp or YYYY-MM-DD date")
}
