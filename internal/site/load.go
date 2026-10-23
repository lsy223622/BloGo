package site

import (
	"sort"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

// LoadContent selects posts by configured categories before validating identities
// and applying visibility. Unclassified posts remain eligible.
func LoadContent(source, taxonomyFile string, location *time.Location, preview bool) ([]*content.ContentItem, error) {
	tax, err := content.ReadTaxonomies(taxonomyFile)
	if err != nil {
		return nil, err
	}
	items, err := content.ReadSources(source)
	if err != nil {
		return nil, err
	}
	selected := make([]*content.ContentItem, 0, len(items))
	for _, item := range items {
		if item.Kind == content.Post && len(item.Meta.Categories) > 0 {
			configured := false
			for _, id := range item.Meta.Categories {
				if _, ok := tax.Categories[id]; ok {
					configured = true
					break
				}
			}
			if !configured {
				continue
			}
		}
		if err := item.Validate(location, tax); err != nil {
			return nil, err
		}
		selected = append(selected, item)
	}
	if err := ValidateRoutes(selected, nil); err != nil {
		return nil, err
	}
	visible := make([]*content.ContentItem, 0, len(selected))
	for _, item := range selected {
		if preview || item.ProductionVisible() {
			visible = append(visible, item)
		}
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	return visible, nil
}
