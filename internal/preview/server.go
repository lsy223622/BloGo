package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/render"
)

const reloadScript = `(() => { let revision; async function check() { try { const response = await fetch('/__blogo/revision', {cache:'no-store'}); if (!response.ok) return; const data = await response.json(); if (revision && revision !== data.build_id) location.reload(); revision = data.build_id; } catch (_) {} } check(); setInterval(check, 1000); })();`

type Handler struct {
	mu     sync.RWMutex
	result *build.Result
	files  map[string]bool
	routes map[string]string
}

func NewHandler(result *build.Result) (*Handler, error) {
	h := &Handler{}
	if err := h.Update(result); err != nil {
		return nil, err
	}
	return h, nil
}
func (h *Handler) Update(result *build.Result) error {
	root, err := os.OpenRoot(filepath.Join(result.Output, "public"))
	if err != nil {
		return err
	}
	defer root.Close()
	files := map[string]bool{}
	if err = fs.WalkDir(root.FS(), ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Type().IsRegular() {
			files[file] = true
		}
		return nil
	}); err != nil {
		return err
	}
	routes := map[string]string{}
	for _, p := range result.Manifest.Pages {
		routes[p.URL] = content.OutputPath(p.URL)
	}
	h.mu.Lock()
	h.result = result
	h.files = files
	h.routes = routes
	h.mu.Unlock()
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/__blogo/revision" {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]string{"build_id": h.result.Manifest.BuildID})
		}
		return
	}
	if r.URL.Path == "/__blogo/dev.js" {
		w.Header().Set("Content-Type", "application/javascript")
		if r.Method == "GET" {
			_, _ = io.WriteString(w, reloadScript)
		}
		return
	}
	if target, ok := h.result.Redirects[r.URL.Path]; ok {
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}
	file, knownPage := h.routes[r.URL.Path]
	status := http.StatusOK
	if r.URL.Path == "/404/" {
		status = http.StatusNotFound
	}
	if !knownPage {
		file = strings.TrimPrefix(r.URL.Path, "/")
		if !fs.ValidPath(file) || strings.Contains(file, "\\") || !h.files[file] {
			file = "404/index.html"
			status = http.StatusNotFound
		}
	}
	root, err := os.OpenRoot(filepath.Join(h.result.Output, "public"))
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	defer root.Close()
	data, err := root.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if path.Ext(file) == ".html" {
		// Reload is an enhancement. A bodyless author document remains readable
		// without injecting script at an unsafe position.
		if enhanced, err := render.PrepareStandalone(string(data), false, `<script src="/__blogo/dev.js" defer></script>`); err == nil {
			data = enhanced
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else if contentType := mime.TypeByExtension(path.Ext(file)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	if r.Method == "GET" {
		_, _ = w.Write(data)
	}
}

func Dev(ctx context.Context, c *config.Config, output, listen string, log io.Writer) error {
	if err := config.Listener(listen); err != nil {
		return err
	}
	result, err := build.Build(c, output, true)
	if err != nil {
		return err
	}
	handler, err := NewHandler(result)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Fprintf(log, "Preview http://%s (%s)\n", listener.Addr(), result.Output)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastSuccessful := result.Manifest.Fingerprint
	failed, lastError := "", ""
	for {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdown)
		case err := <-done:
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		case <-ticker.C:
			model, err := config.Load(c.FilePath())
			if err != nil {
				if err.Error() != lastError {
					fmt.Fprintf(log, "dev: %v\n", err)
					lastError = err.Error()
				}
				continue
			}
			fingerprint, err := build.Scan(model, true)
			if err != nil {
				if err.Error() != lastError {
					fmt.Fprintf(log, "dev: %v\n", err)
					lastError = err.Error()
				}
				continue
			}
			if fingerprint == lastSuccessful || fingerprint == failed {
				continue
			}
			updated, err := build.Build(model, output, true)
			if err != nil {
				if _, ok := err.(*build.ValidationError); ok {
					failed = fingerprint
				}
				if err.Error() != lastError {
					fmt.Fprintf(log, "dev: %v\n", err)
					lastError = err.Error()
				}
				continue
			}
			if err = handler.Update(updated); err != nil {
				fmt.Fprintf(log, "dev: %v\n", err)
				continue
			}
			lastSuccessful = updated.Manifest.Fingerprint
			failed = ""
			lastError = ""
			fmt.Fprintf(log, "Rebuilt %s\n", updated.Manifest.BuildID)
		}
	}
}
