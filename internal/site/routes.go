package site

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/lsy223622/BloGo/internal/content"
)

var pagination = regexp.MustCompile(`(?:^|/)page/[0-9]+(?:/|$)`)
var archiveRoute = regexp.MustCompile(`^/archives/[0-9]{4}(?:/[0-9]{2})?/$`)

func reserved(u string) bool {
	if strings.HasSuffix(u, "/index.html") {
		u = strings.TrimSuffix(u, "index.html")
	}
	for _, prefix := range []string{"/api/", "/data/", "/assets/"} {
		if u == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(u, prefix) {
			return true
		}
	}
	if pagination.MatchString(u) || archiveRoute.MatchString(u) {
		return true
	}
	parts := strings.Split(strings.Trim(u, "/"), "/")
	if len(parts) == 1 {
		switch parts[0] {
		case "", "categories", "tags", "archives", "updates", "404", "rss.xml", "sitemap.xml", "robots.txt":
			return true
		}
	}
	return len(parts) == 2 && (parts[0] == "categories" || parts[0] == "tags")
}

// ValidateRoutes checks one collection before any output is written. staticFiles
// are slash-separated paths relative to the static root, never absolute paths.
func ValidateRoutes(items []*content.ContentItem, staticFiles []string) error {
	ids := map[string]string{}
	routes := map[string]string{}
	files := map[string]string{}
	addFile := func(file, owner string) error {
		for other, previous := range files {
			if file == other || strings.HasPrefix(file, other+"/") || strings.HasPrefix(other, file+"/") {
				return fmt.Errorf("%s: output %q conflicts with %s (%s)", owner, file, previous, other)
			}
		}
		files[file] = owner
		return nil
	}
	for _, file := range []string{"index.html", "categories/index.html", "tags/index.html", "archives/index.html", "updates/index.html", "404/index.html", "rss.xml", "sitemap.xml", "robots.txt"} {
		if err := addFile(file, "system route"); err != nil {
			return err
		}
	}
	addRoute := func(u, owner string) error {
		if reserved(u) {
			return fmt.Errorf("%s: reserved route %s", owner, u)
		}
		key := strings.TrimSuffix(u, "/")
		if previous, ok := routes[key]; ok {
			return fmt.Errorf("%s: route %s conflicts with %s", owner, u, previous)
		}
		routes[key] = owner
		file := strings.TrimPrefix(u, "/")
		if strings.HasSuffix(u, "/") {
			file = content.OutputPath(u)
		}
		return addFile(file, owner)
	}
	for _, item := range items {
		if item.ID == "" || item.URL == "" {
			return fmt.Errorf("%s: unresolved identity", item.SourcePath)
		}
		if previous, ok := ids[item.ID]; ok {
			return fmt.Errorf("%s: duplicate ID %s from %s", item.SourcePath, item.ID, previous)
		}
		ids[item.ID] = item.SourcePath
		if err := addRoute(item.URL, item.SourcePath); err != nil {
			return err
		}
		for _, alias := range item.Aliases {
			if err := addRoute(alias, item.SourcePath+" aliases"); err != nil {
				return err
			}
		}
	}
	for _, file := range staticFiles {
		if !fs.ValidPath(file) || strings.Contains(file, "\\") {
			return fmt.Errorf("invalid static output path %q", file)
		}
		u := "/" + file
		if strings.HasPrefix(u, "/api/") || strings.HasPrefix(u, "/data/") || u == "/api" || u == "/data" {
			return fmt.Errorf("static %s: reserved service mapping", file)
		}
		if path.Base(u) == "index.html" && reserved(u) && !strings.HasPrefix(u, "/assets/") {
			return fmt.Errorf("static %s: reserved system route", file)
		}
		if !strings.HasPrefix(u, "/assets/") && (reserved(u+"/") || u == "/page" || strings.HasSuffix(u, "/page")) {
			return fmt.Errorf("static %s: reserved output directory", file)
		}
		if err := addFile(file, "static "+file); err != nil {
			return err
		}
	}
	return nil
}
