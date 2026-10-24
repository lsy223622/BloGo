package content

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/lsy223622/BloGo/internal/yamlutil"
)

type Profile struct {
	Type  string `yaml:"type"`
	Label string `yaml:"label"`
	URL   string `yaml:"url"`
}

type Settings struct {
	SchemaVersion   int                 `yaml:"schema_version"`
	Title           string              `yaml:"title"`
	Subtitle        string              `yaml:"subtitle"`
	Language        string              `yaml:"language"`
	PageSize        int                 `yaml:"page_size"`
	SummaryMaxChars int                 `yaml:"summary_max_chars"`
	Logo            string              `yaml:"logo"`
	Favicon         string              `yaml:"favicon"`
	Profile         []Profile           `yaml:"profile"`
	Categories      map[string]Category `yaml:"categories"`
	Tags            map[string]Tag      `yaml:"tags"`
	Extra           map[string]any      `yaml:"extra"`
}

func (s *Settings) Taxonomies() *Taxonomies {
	return &Taxonomies{SchemaVersion: s.SchemaVersion, Categories: s.Categories, Tags: s.Tags, Extra: s.Extra}
}

func ReadSettings(file string) (*Settings, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: expected ordinary settings file", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ParseSettings(file, data)
}

func ParseSettings(file string, data []byte) (*Settings, error) {
	s := &Settings{Language: "zh-CN", PageSize: 10, SummaryMaxChars: 160, Logo: "/images/logo.svg", Favicon: "/images/favicon.svg"}
	n, err := yamlutil.Decode(data, s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if _, err = validateTaxonomies(file, s.Taxonomies(), n); err != nil {
		return nil, err
	}
	if s.PageSize < 1 || s.SummaryMaxChars < 1 {
		return nil, fmt.Errorf("%s: page_size and summary_max_chars must be positive", file)
	}
	for _, resource := range []string{s.Logo, s.Favicon} {
		if !strings.HasPrefix(resource, "/") || !fs.ValidPath(strings.TrimPrefix(resource, "/")) || strings.ContainsAny(resource, "?#%\\") {
			return nil, fmt.Errorf("%s: logo and favicon must be absolute local resource paths", file)
		}
	}
	for _, p := range s.Profile {
		switch p.Type {
		case "qq", "email", "github", "website", "cc":
		default:
			return nil, fmt.Errorf("%s: unknown profile type %q", file, p.Type)
		}
	}
	return s, nil
}
