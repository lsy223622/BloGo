package site

import (
	"sort"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

// LoadContent validates one source collection before applying production
// visibility. Stable-ID order makes undated pages independent of discovery order.
func LoadContent(source, taxonomyFile string, location *time.Location, preview bool) ([]*content.ContentItem, error) {
	tax, err := content.ReadTaxonomies(taxonomyFile)
	if err != nil {
		return nil, err
	}
	items, err := content.ReadSources(source)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := item.Validate(location, tax); err != nil {
			return nil, err
		}
	}
	if err := ValidateRoutes(items, nil); err != nil {
		return nil, err
	}
	visible := make([]*content.ContentItem, 0, len(items))
	for _, item := range items {
		if preview || item.ProductionVisible() {
			visible = append(visible, item)
		}
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	return visible, nil
}
