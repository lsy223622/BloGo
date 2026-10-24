package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/render"
	"github.com/lsy223622/BloGo/internal/requestmeta"
	"github.com/lsy223622/BloGo/internal/site"
	"github.com/lsy223622/BloGo/internal/stats"
)

const reloadScript = `(() => {let revision;async function check(){try{const response=await fetch('/__blogo/revision',{cache:'no-store'});if(!response.ok)return;const data=await response.json();if(revision&&revision!==data.build_id)location.reload();revision=data.build_id;}catch(_) {}}check();setInterval(check,1000);})();`

type httpSnapshot struct {
	config    config.Config
	published *build.Result
	files     map[string]bool
	routes    map[string]string
	records   map[string]site.PageRecord
	proxies   []netip.Prefix
	collector http.Handler
}
type Handler struct {
	mu      sync.RWMutex
	current *httpSnapshot
	preview bool
}

func NewHandler(c *config.Config, published *build.Result, collector http.Handler) (*Handler, error) {
	return newHandler(c, published, collector, false)
}
func NewPreviewHandler(published *build.Result) (*Handler, error) {
	return newHandler(&config.Config{}, published, nil, true)
}
func newHandler(c *config.Config, published *build.Result, collector http.Handler, preview bool) (*Handler, error) {
	h := &Handler{preview: preview}
	if err := h.Reconfigure(c, published, collector); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Handler) Reconfigure(c *config.Config, published *build.Result, collector http.Handler) error {
	if published == nil || published.Manifest == nil {
		return fmt.Errorf("published manifest required")
	}
	root, err := os.OpenRoot(filepath.Join(published.Output, "public"))
	if err != nil {
		return err
	}
	defer root.Close()
	proxies, err := TrustedProxies(c.Server.TrustedProxies)
	if err != nil {
		return err
	}
	result := *published
	manifest := *published.Manifest
	manifest.Pages = append([]site.PageRecord(nil), manifest.Pages...)
	result.Manifest = &manifest
	result.Redirects = map[string]string{}
	for source, target := range published.Redirects {
		result.Redirects[source] = target
	}
	state := &httpSnapshot{config: *c, published: &result, files: map[string]bool{}, routes: map[string]string{}, records: map[string]site.PageRecord{}, proxies: proxies, collector: collector}
	if err = fs.WalkDir(root.FS(), ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in public output")
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("nonregular public file")
			}
			state.files[file] = true
		}
		return nil
	}); err != nil {
		return err
	}
	for _, page := range manifest.Pages {
		state.routes[page.URL] = content.OutputPath(page.URL)
		state.records[page.ID] = page
	}
	h.mu.Lock()
	h.current = state
	h.mu.Unlock()
	return nil
}

func (h *Handler) Update(published *build.Result) error {
	h.mu.RLock()
	c, collector := h.current.config, h.current.collector
	h.mu.RUnlock()
	return h.Reconfigure(&c, published, collector)
}

func privatePath(value string) bool {
	relative := strings.TrimPrefix(value, "/")
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\\x00") || relative != "" && !fs.ValidPath(strings.TrimSuffix(relative, "/")) {
		return true
	}
	for _, segment := range strings.Split(relative, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	first := strings.ToLower(strings.SplitN(relative, "/", 2)[0])
	switch first {
	case "config.yaml", "content", "data", "meta", "statistics", "private", "templates", "static", "__blogo":
		return true
	}
	return false
}

var immutableAsset = regexp.MustCompile(`^assets/[^/]+\.[a-f0-9]{16}\.(css|js)$`)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	state := h.current
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	if r.URL.Path == "/api/events" {
		if !state.config.Stats.Enabled || state.collector == nil {
			http.NotFound(w, r)
			return
		}
		ip, err := ClientIP(r, state.proxies)
		if err != nil {
			http.Error(w, "invalid peer", 400)
			return
		}
		state.collector.ServeHTTP(w, requestmeta.WithClientIP(r, ip))
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.URL.Path == "/data/stats.json" {
		if !state.config.Stats.Enabled {
			http.NotFound(w, r)
			return
		}
		servePublicReport(w, r, state)
		return
	}
	if h.preview && r.URL.Path == "/__blogo/revision" {
		data, _ := json.Marshal(map[string]string{"build_id": state.published.Manifest.BuildID})
		serveBytes(w, r, data, "application/json")
		return
	}
	if h.preview && r.URL.Path == "/__blogo/dev.js" {
		serveBytes(w, r, []byte(reloadScript), "application/javascript")
		return
	}
	denied := privatePath(r.URL.Path)
	if !denied {
		if target, ok := state.published.Redirects[r.URL.Path]; ok {
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
	}
	file, known := state.routes[r.URL.Path]
	status := 200
	if denied {
		known = false
	}
	if !known {
		file = strings.TrimPrefix(r.URL.Path, "/")
		if denied || !state.files[file] {
			file = "404/index.html"
			status = 404
		}
	}
	if r.URL.Path == "/404/" {
		status = 404
	}
	root, err := os.OpenRoot(filepath.Join(state.published.Output, "public"))
	if err != nil {
		http.Error(w, "site unavailable", 503)
		return
	}
	defer root.Close()
	f, err := root.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	if path.Ext(file) == ".html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if h.preview {
			data, err := io.ReadAll(f)
			if err != nil {
				http.Error(w, "site unavailable", 503)
				return
			}
			if enhanced, err := render.PrepareStandalone(string(data), false, `<script src="/__blogo/dev.js" defer></script>`); err == nil {
				data = enhanced
			}
			w.WriteHeader(status)
			if r.Method == "GET" {
				w.Write(data)
			}
			return
		}
	} else if kind := mime.TypeByExtension(path.Ext(file)); kind != "" {
		w.Header().Set("Content-Type", kind)
	}
	if immutableAsset.MatchString(file) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if status != 200 {
		w.WriteHeader(status)
		if r.Method == "GET" {
			io.Copy(w, f)
		}
		return
	}
	http.ServeContent(w, r, file, info.ModTime(), f)
}

func serveBytes(w http.ResponseWriter, r *http.Request, data []byte, kind string) {
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(200)
	if r.Method == "GET" {
		w.Write(data)
	}
}

func servePublicReport(w http.ResponseWriter, r *http.Request, state *httpSnapshot) {
	raw, err := os.ReadFile(state.config.Stats.PublicOutput)
	if err != nil {
		http.Error(w, "statistics unavailable", 503)
		return
	}
	var report stats.PublicReport
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&report); err != nil || decoder.Decode(new(any)) != io.EOF || report.SchemaVersion != 1 || report.BuildID != state.published.Manifest.BuildID {
		http.Error(w, "statistics unavailable", 503)
		return
	}
	for id, count := range report.Pages {
		p, ok := state.records[id]
		if !ok || p.Visibility != content.Public || !p.StatsEnabled || count.Views < 0 {
			delete(report.Pages, id)
		}
	}
	popular := []stats.PopularPage{}
	for _, item := range report.Popular30D {
		p, ok := state.records[item.ID]
		if ok && p.Visibility == content.Public && p.StatsEnabled && p.Kind == "post" && item.Views >= 0 {
			popular = append(popular, stats.PopularPage{ID: p.ID, Title: p.Title, URL: p.URL, Views: item.Views})
		}
	}
	report.Popular30D = popular
	data, err := json.Marshal(report)
	if err != nil {
		http.Error(w, "statistics unavailable", 503)
		return
	}
	serveBytes(w, r, data, "application/json")
}
