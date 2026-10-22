package preview

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/site"
)

func TestPreviewCanonicalRoutesDraftsAndPrivateFiles(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := build.Build(c, filepath.Join(t.TempDir(), "preview"), true)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path     string
		status   int
		location string
	}{{"/posts/welcome?x=1", 301, "/posts/welcome/?x=1"}, {"/posts/welcome/index.html?q=2", 301, "/posts/welcome/?q=2"}, {"/index.html", 301, "/"}, {"/posts/draft/", 200, ""}, {"/lab/", 200, ""}, {"/missing/", 404, ""}, {"/meta/manifest.json", 404, ""}, {"/%2e%2e/config.yaml", 404, ""}, {"/images/go.svg", 200, ""}} {
		t.Run(tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", tc.path, nil))
			if response.Code != tc.status || response.Header().Get("Location") != tc.location {
				t.Fatalf("%d %s %s", response.Code, response.Header().Get("Location"), response.Body)
			}
			if tc.path == "/posts/draft/" && !strings.Contains(response.Body.String(), "/__blogo/dev.js") {
				t.Fatal("live reload absent")
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/404/", nil))
	if response.Code != 404 {
		t.Fatal("404 page returned success")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/", nil))
	if response.Code != 405 {
		t.Fatal("unexpected method accepted")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/__blogo/revision", nil))
	if !strings.Contains(response.Body.String(), result.Manifest.BuildID) {
		t.Fatal("revision mismatch")
	}
}

func TestDevRejectsNonLoopbackBeforeBuild(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err = Dev(context.Background(), c, out, "0.0.0.0:8080", io.Discard); err == nil {
		t.Fatal("public preview listener accepted")
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 0 {
		t.Fatal("listener rejection occurred after build")
	}
}

func TestPreviewPreservesBodylessAuthorDocument(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "public/custom/index.html")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	body := `<!doctype html><title>Custom</title><p>Author source`
	if err := os.WriteFile(file, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(&build.Result{Output: root, Manifest: &site.Manifest{Pages: []site.PageRecord{{URL: "/custom/"}}}})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("GET", "/custom/", nil))
	if response.Code != 200 || response.Body.String() != body {
		t.Fatal("author document corrupted")
	}
}
