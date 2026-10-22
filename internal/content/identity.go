package content

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/lsy223622/BloGo/internal/yamlutil"
)

var stableID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var aliasSegment = regexp.MustCompile(`^[A-Za-z0-9_~.-]+$`)

func ValidID(id string) bool { return stableID.MatchString(id) }

func (item *ContentItem) ResolveIdentity() error {
	slug := item.Meta.Slug
	if yamlutil.Field(item.node, "slug") == item.node {
		name := strings.TrimSuffix(path.Base(item.SourcePath), path.Ext(item.SourcePath))
		var b strings.Builder
		for _, r := range name {
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			if r == ' ' || r == '_' {
				r = '-'
			}
			if r == '-' && (b.Len() == 0 || strings.HasSuffix(b.String(), "-")) {
				continue
			}
			b.WriteRune(r)
		}
		slug = strings.Trim(b.String(), "-")
	}
	if !ValidID(slug) {
		return item.fieldError("slug", "expected [a-z0-9]+(?:-[a-z0-9]+)*; supply an explicit ASCII slug for this filename")
	}
	id := item.Meta.ID
	if yamlutil.Field(item.node, "id") == item.node {
		id = slug
	}
	if !ValidID(id) {
		return item.fieldError("id", "invalid ASCII stable ID")
	}
	item.Slug = slug
	item.ID = string(item.Kind) + ":" + id
	u := item.Meta.Permalink
	if yamlutil.Field(item.node, "permalink") == item.node {
		if item.Kind == Post {
			u = "/posts/" + slug + "/"
		} else {
			parent := strings.TrimPrefix(path.Dir(item.SourcePath), "pages")
			if parent != "" {
				for _, segment := range strings.Split(strings.TrimPrefix(parent, "/"), "/") {
					if !ValidID(segment) {
						return item.fieldError("permalink", "source parent directories require valid slug segments or an explicit permalink")
					}
				}
			}
			u = parent + "/" + slug + "/"
		}
	}
	canonical, err := CanonicalURL(u)
	if err != nil {
		return item.fieldError("permalink", err.Error())
	}
	item.URL = canonical
	item.OutputPath = OutputPath(canonical)
	item.Aliases = nil
	for _, alias := range item.Meta.Aliases {
		u, err := AliasURL(alias)
		if err != nil {
			return item.fieldError("aliases", err.Error())
		}
		item.Aliases = append(item.Aliases, u)
	}
	return nil
}

func CanonicalURL(raw string) (string, error) {
	if raw == "/" {
		return raw, nil
	}
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("expected site-root path")
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(raw, "/"), "/")
	for _, part := range strings.Split(trimmed, "/") {
		if !ValidID(part) {
			return "", fmt.Errorf("invalid canonical route segment %q", part)
		}
	}
	return "/" + trimmed + "/", nil
}

func AliasURL(raw string) (string, error) {
	if raw == "/" {
		return raw, nil
	}
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("alias must be a site-root path")
	}
	trimmed := strings.TrimSuffix(raw[1:], "/")
	for _, part := range strings.Split(trimmed, "/") {
		if !aliasSegment.MatchString(part) || part == "." || part == ".." {
			return "", fmt.Errorf("invalid alias path segment %q", part)
		}
	}
	if strings.Contains(path.Base(trimmed), ".") {
		if strings.HasSuffix(raw, "/") {
			return "", fmt.Errorf("file alias cannot have a trailing slash")
		}
		return raw, nil
	}
	return "/" + trimmed + "/", nil
}

func OutputPath(canonical string) string { return strings.TrimPrefix(canonical, "/") + "index.html" }
