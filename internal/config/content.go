package config

import (
	"fmt"
	"github.com/lsy223622/BloGo/internal/content"
	"strings"
)

type ContentSettings = content.Settings

func LoadContentSettings(file string) (*ContentSettings, error) {
	s, err := content.ReadSettings(file)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.Title) == "" {
		return nil, fmt.Errorf("%s: title: required", file)
	}
	return s, nil
}

func (c *Config) ApplyContentSettings(settings *ContentSettings) {
	c.Content = settings
	c.Site.Title = settings.Title
	c.Site.Subtitle = settings.Subtitle
	c.Site.Language = settings.Language
	c.Build.PageSize = settings.PageSize
	c.Build.SummaryMaxChars = settings.SummaryMaxChars
	profile := []any{}
	for _, p := range settings.Profile {
		profile = append(profile, map[string]any{"type": p.Type, "label": p.Label, "url": p.URL})
	}
	c.Extra = map[string]any{"profile": profile}
}
