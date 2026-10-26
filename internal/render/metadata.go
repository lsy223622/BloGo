package render

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/lsy223622/BloGo/internal/content"
)

type blogPosting struct {
	Context     string `json:"@context"`
	Type        string `json:"@type"`
	Headline    string `json:"headline"`
	URL         string `json:"url"`
	Published   string `json:"datePublished"`
	Modified    string `json:"dateModified,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
}

func (r *Renderer) metadata(v *view) error {
	p := v.Page
	home := p.ID == "home" || strings.HasPrefix(p.ID, "home:page:")
	title := p.Title
	description := v.Summary
	if description == "" {
		description = r.config.Site.Subtitle
	}
	if description == "" {
		description = r.config.Site.Title
	}
	parts := strings.Split(p.ID, ":")
	if home {
		title = r.config.Site.Title
		description = r.config.Site.Subtitle
		if description == "" {
			description = r.config.Site.Title
		}
	}
	if r.tax != nil && len(parts) > 1 {
		switch parts[0] {
		case "category":
			title += " · 分类"
			if term := r.tax.Categories[parts[1]]; term.Description != "" {
				description = term.Description
			}
		case "tag":
			title += " · 标签"
			if term := r.tax.Tags[parts[1]]; term.Description != "" {
				description = term.Description
			}
		}
	}
	if p.Pager.Total > 1 {
		title += fmt.Sprintf(" · 第 %d 页", p.Pager.Number)
	}
	v.MetaTitle, v.Description = title, description
	v.DocumentTitle = title
	if !home {
		v.DocumentTitle += " · " + r.config.Site.Title
	}
	if p.Content == nil || p.Content.Kind != content.Post || p.Visibility != content.Public || p.Published == nil {
		return nil
	}
	posting := blogPosting{Context: "https://schema.org", Type: "BlogPosting", Headline: p.Title, URL: v.Canonical, Published: p.Published.Format(time.RFC3339), Description: v.Summary}
	if p.Content.Meta.Updated != nil && p.Content.Updated != nil {
		posting.Modified = p.Content.Updated.Format(time.RFC3339)
	}
	if cover, ok := p.Content.Meta.Extra["cover"].(string); ok && cover != "" {
		base, err := url.Parse(v.Canonical)
		if err != nil {
			return err
		}
		reference, err := url.Parse(cover)
		if err != nil {
			return err
		}
		image := base.ResolveReference(reference)
		if (image.Scheme == "http" || image.Scheme == "https") && image.Host != "" {
			posting.Image = image.String()
		}
	}
	data, err := json.Marshal(posting)
	if err != nil {
		return err
	}
	// Marshal's default HTML escaping prevents author text from closing the script.
	v.StructuredData = template.JS(data)
	return nil
}
