package build

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/markdown"
	"github.com/lsy223622/BloGo/internal/site"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/parse/v2"
	cssparse "github.com/tdewolff/parse/v2/css"
)

var staticExtensions = map[string]bool{".css": true, ".js": true, ".json": true, ".xml": true, ".txt": true, ".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".avif": true, ".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".pdf": true, ".webm": true, ".mp4": true, ".mp3": true, ".ogg": true}

func staticAllowed(file string) bool {
	return staticExtensions[strings.ToLower(path.Ext(file))] || path.Base(file) == "LICENSE"
}

func readTree(root string, allowed func(string) bool) (map[string][]byte, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s: expected ordinary directory", root)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	files := map[string][]byte{}
	err = fs.WalkDir(r.FS(), ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: symbolic links are not allowed", file)
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !allowed(file) {
			return fmt.Errorf("%s: unsupported input type", file)
		}
		data, err := r.ReadFile(file)
		if err != nil {
			return err
		}
		files[file] = data
		return nil
	})
	return files, err
}

func CopyAssets(source, public string, pages []*site.Page) (map[string]string, error) {
	files, err := readTree(source, staticAllowed)
	if err != nil {
		return nil, err
	}
	for _, page := range pages {
		if page.StatsEnabled {
			if _, ok := files["assets/stats.js"]; !ok {
				return nil, fmt.Errorf("assets/stats.js: required for pages with statistics enabled")
			}
			break
		}
	}
	assets := map[string]string{}
	generated := map[string][]byte{}
	managed := map[string][]byte{"highlight.css": []byte(markdown.HighlightCSS())}
	for _, name := range []string{"theme.css", "theme.js", "stats.js"} {
		if data, ok := files["assets/"+name]; ok {
			managed[name] = data
		}
	}
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)
	for _, name := range fileKeys(managed) {
		data := managed[name]
		mime := "text/css"
		if path.Ext(name) == ".js" {
			mime = "application/javascript"
		}
		compact, err := m.Bytes(mime, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		hash := sha256.Sum256(compact)
		ext := path.Ext(name)
		file := "assets/" + strings.TrimSuffix(name, ext) + fmt.Sprintf(".%x", hash[:8]) + ext
		generated[file] = compact
		assets[name] = "/" + file
	}
	for _, entry := range []struct{ key, file string }{{"katex.css", "vendor/katex-0.19.0/katex.min.css"}, {"katex.js", "vendor/katex-0.19.0/katex.min.js"}, {"mermaid.js", "vendor/mermaid-12.2.0/mermaid.min.js"}} {
		if _, ok := files[entry.file]; ok {
			assets[entry.key] = "/" + entry.file
		}
	}
	var items []*content.ContentItem
	for _, p := range pages {
		if p.Content != nil {
			items = append(items, p.Content)
		}
	}
	names := append(fileKeys(files), fileKeys(generated)...)
	if err = site.ValidateRoutes(items, names); err != nil {
		return nil, err
	}
	for _, p := range pages {
		pageFile := content.OutputPath(p.URL)
		for _, file := range names {
			if file == pageFile || strings.HasPrefix(file, pageFile+"/") || strings.HasPrefix(pageFile, file+"/") {
				return nil, fmt.Errorf("static %s conflicts with page %s", file, p.URL)
			}
		}
	}
	all := map[string][]byte{}
	for _, group := range []map[string][]byte{files, generated} {
		for name, data := range group {
			all[name] = data
		}
	}
	for _, name := range fileKeys(all) {
		if path.Ext(name) == ".css" {
			if err := checkCSSReferences(name, all[name], all); err != nil {
				return nil, err
			}
		}
	}
	for _, group := range []map[string][]byte{files, generated} {
		for _, file := range fileKeys(group) {
			if err = writeFile(public, file, group[file]); err != nil {
				return nil, err
			}
		}
	}
	return assets, nil
}

func checkCSSReferences(file string, data []byte, files map[string][]byte) error {
	l := cssparse.NewLexer(parse.NewInput(bytes.NewReader(data)))
	for {
		kind, raw := l.Next()
		if kind == cssparse.ErrorToken {
			if l.Err() != io.EOF {
				return fmt.Errorf("%s: %w", file, l.Err())
			}
			return nil
		}
		if kind == cssparse.BadURLToken {
			return fmt.Errorf("%s: malformed resource URL", file)
		}
		if kind != cssparse.URLToken {
			continue
		}
		value := strings.TrimSpace(string(raw))
		open := strings.IndexByte(value, '(')
		value = strings.TrimSpace(value[open+1 : len(value)-1])
		value = strings.Trim(value, "\"'")
		if err := checkLocalResource(file, value, files); err != nil {
			return err
		}
	}
}

func checkLocalResource(file, raw string, files map[string][]byte) error {
	ref, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: invalid resource %q", file, raw)
	}
	if ref.IsAbs() || ref.Host != "" || ref.Path == "" {
		return nil
	}
	if strings.Contains(ref.Path, "\\") {
		return fmt.Errorf("%s: invalid resource path %q", file, raw)
	}
	target := ref.Path
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	} else {
		target = path.Join(path.Dir(file), target)
	}
	if !fs.ValidPath(target) {
		return fmt.Errorf("%s: resource escapes public root %q", file, raw)
	}
	if _, ok := files[target]; !ok {
		return fmt.Errorf("%s: missing resource %q", file, raw)
	}
	return nil
}

func fileKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func writeFile(root, file string, data []byte) error {
	if !fs.ValidPath(file) || strings.Contains(file, "\\") {
		return fmt.Errorf("invalid output path %q", file)
	}
	full := filepath.Join(root, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0644)
}
