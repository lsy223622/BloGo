package content

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/lsy223622/BloGo/internal/yamlutil"
	"go.yaml.in/yaml/v3"
)

type Taxonomies struct {
	SchemaVersion int                 `yaml:"schema_version"`
	Categories    map[string]Category `yaml:"categories"`
	Tags          map[string]Tag      `yaml:"tags"`
	Extra         map[string]any      `yaml:"extra"`
}
type Category struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Parent      string         `yaml:"parent"`
	Extra       map[string]any `yaml:"extra"`
}
type Tag struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Extra       map[string]any `yaml:"extra"`
}

func ReadTaxonomies(file string) (*Taxonomies, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: expected ordinary taxonomy file", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ParseTaxonomies(file, data)
}

func ParseTaxonomies(file string, data []byte) (*Taxonomies, error) {
	settings, err := ParseSettings(file, data)
	if err != nil {
		return nil, err
	}
	return settings.Taxonomies(), nil
}

func validateTaxonomies(file string, value *Taxonomies, n *yaml.Node) (*Taxonomies, error) {
	tax := *value
	fail := func(section, id, field, message string) (*Taxonomies, error) {
		node := yamlutil.Field(n, section)
		if id != "" {
			node = yamlutil.Field(node, id)
		}
		node = yamlutil.Field(node, field)
		return nil, fmt.Errorf("%s: %w", file, yamlutil.Error(node, strings.Trim(section+"."+id+"."+field, "."), message))
	}
	if tax.SchemaVersion != 1 {
		return fail("schema_version", "", "", "must be 1")
	}
	for _, id := range sortedKeys(tax.Categories) {
		c := tax.Categories[id]
		if !ValidID(id) {
			return fail("categories", id, "", "invalid ASCII stable ID")
		}
		if !displayName(c.Name) {
			return fail("categories", id, "name", "expected nonblank name without control characters")
		}
		entry := yamlutil.Field(yamlutil.Field(n, "categories"), id)
		if yamlutil.Field(entry, "parent") != entry {
			if !ValidID(c.Parent) {
				return fail("categories", id, "parent", "invalid ASCII stable ID")
			}
		}
		if c.Parent != "" {
			if _, ok := tax.Categories[c.Parent]; !ok {
				return fail("categories", id, "parent", "unknown parent "+c.Parent)
			}
		}
	}
	for _, id := range sortedKeys(tax.Tags) {
		tag := tax.Tags[id]
		if !ValidID(id) {
			return fail("tags", id, "", "invalid ASCII stable ID")
		}
		if !displayName(tag.Name) {
			return fail("tags", id, "name", "expected nonblank name without control characters")
		}
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("category parent cycle at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		if parent := tax.Categories[id].Parent; parent != "" {
			if err := visit(parent); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for _, id := range sortedKeys(tax.Categories) {
		if err := visit(id); err != nil {
			return fail("categories", id, "parent", err.Error())
		}
	}
	return &tax, nil
}

func displayName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
