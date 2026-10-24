package server

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
)

func TestServiceShutdownAndHotRebuild(t *testing.T) {
	c := runtimeFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	c.Server.ListenAddr = address
	raw, _ := os.ReadFile(c.FilePath())
	putRuntime(t, c.FilePath(), strings.Replace(string(raw), "server:\n", "server:\n  listen_addr: "+address+"\n", 1))
	c, err = config.Load(c.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		if runtime.GOOS == "linux" {
			done <- Serve(ctx, c, io.Discard)
		} else {
			done <- Dev(ctx, c, "", address, io.Discard)
		}
	}()
	client := http.Client{Timeout: time.Second}
	waitBody := func(want string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			response, e := client.Get("http://" + address + "/posts/a/")
			if e == nil {
				body, _ := io.ReadAll(response.Body)
				response.Body.Close()
				if response.StatusCode == 200 && strings.Contains(string(body), want) {
					return
				}
			}
			select {
			case e := <-done:
				t.Fatalf("service exited: %v", e)
			default:
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("did not serve %s", want)
	}
	waitBody("Body.")
	putRuntime(t, filepath.Join(c.Paths.Source, "posts/a.md"), "---\nid: a\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\nChanged body.")
	waitBody("Changed body.")
	cancel()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("service did not shut down")
	}
}

func runtimeFixture(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	templates, _ := filepath.Abs("../../templates")
	static, _ := filepath.Abs("../../static")
	raw := "server:\n  base_url: https://example.com\nbuild:\n  scan_interval: 30ms\nstats:\n  report_interval: 40ms\npaths:\n  templates: " + filepath.ToSlash(templates) + "\n  static: " + filepath.ToSlash(static) + "\n"
	putRuntime(t, filepath.Join(root, "config.yaml"), raw)
	putRuntime(t, filepath.Join(root, "content/site.yaml"), "schema_version: 1\ntitle: Test\n")
	putRuntime(t, filepath.Join(root, "content/posts/a.md"), "---\nid: a\ntitle: A\nstatus: public\npublished: 2026-10-23\nstats: true\n---\nBody.")
	putRuntime(t, filepath.Join(root, "content/posts/draft.md"), "---\nid: draft\ntitle: Draft\nstatus: draft\nstats: true\n---\nDraft.")
	c, err := config.Load(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func putRuntime(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDevStatsExcludeDrafts(t *testing.T) {
	c := runtimeFixture(t)
	r, err := startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	if r.config.Site.BaseURL != "http://127.0.0.1:25646" || !strings.Contains(r.config.Stats.DatabasePath, filepath.Join("data", "dev")) {
		t.Fatal("dev context", r.config)
	}
	w := httptest.NewRecorder()
	r.handler.ServeHTTP(w, httptest.NewRequest("GET", "/posts/draft/", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	manifest, err := site.ReadManifest(r.config.Stats.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range manifest.Pages {
		if p.Visibility == content.Draft {
			t.Fatal("draft in collector manifest")
		}
	}
	for _, test := range []struct {
		id   string
		want int
	}{{"post:a", 204}, {"post:draft", 400}, {"home", 204}} {
		body := `{"v":1,"event_id":"` + strings.Repeat("a", 31) + string(rune('0'+test.want%10)) + `","page_id":"` + test.id + `","visitor_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","referrer_host":""}`
		req := httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.handler.ServeHTTP(w, req)
		if w.Code != test.want {
			t.Fatalf("%s: %d %s", test.id, w.Code, w.Body)
		}
	}
}
func TestServeFirstRunAndRestart(t *testing.T) {
	c := runtimeFixture(t)
	r, err := startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(r.config.Stats.VisitorKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if second, e := startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", io.Discard); e == nil {
		second.close()
		t.Fatal("second runtime accepted")
	}
	database := r.config.Stats.DatabasePath
	keyPath := r.config.Stats.VisitorKeyPath
	r.close()
	r, err = startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := os.ReadFile(r.config.Stats.VisitorKeyPath)
	if !bytes.Equal(key, next) {
		t.Fatal("key replaced")
	}
	r.close()
	if err = os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	if info, e := os.Stat(database); e != nil || info.Size() == 0 {
		t.Fatal("missing database")
	}
	if r, err = startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", io.Discard); err == nil {
		r.close()
		t.Fatal("missing key recreated for existing database")
	}
}
func TestServeReloadAndSchedule(t *testing.T) {
	c := runtimeFixture(t)
	var log bytes.Buffer
	r, err := startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", &log)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	original := r.result.Manifest.BuildID
	putRuntime(t, filepath.Join(c.Paths.Source, "posts/a.md"), "invalid content")
	r.step(context.Background(), time.Now().Add(time.Second))
	if r.result.Manifest.BuildID != original {
		t.Fatal("failed build replaced site")
	}
	raw, _ := os.ReadFile(c.FilePath())
	updated := strings.Replace(string(raw), "30ms", "10ms", 1) + ""
	updated = strings.Replace(updated, "report_interval: 40ms", "report_interval: 20ms\n  details:\n    enabled: true\n    retention: 1h", 1)
	putRuntime(t, c.FilePath(), updated)
	r.step(context.Background(), time.Now().Add(2*time.Second))
	if time.Duration(r.config.Build.ScanInterval) != 10*time.Millisecond || !r.config.Stats.Details.Enabled || time.Duration(r.config.Stats.Details.Retention) != time.Hour {
		t.Fatal("mutable settings not applied")
	}
	putRuntime(t, c.FilePath(), strings.Replace(updated, "https://example.com", "https://changed.example", 1))
	r.step(context.Background(), time.Now().Add(3*time.Second))
	if r.config.Server.BaseURL != "https://example.com" || !strings.Contains(log.String(), "restart") {
		t.Fatal("restart settings partly applied", log.String())
	}
	putRuntime(t, c.FilePath(), "bad: [")
	r.step(context.Background(), time.Now().Add(4*time.Second))
	if !r.config.Stats.Details.Enabled {
		t.Fatal("bad YAML replaced settings")
	}
}

func TestFailedHTTPReconfigureKeepsRuntimeSnapshot(t *testing.T) {
	c := runtimeFixture(t)
	var log bytes.Buffer
	r, err := startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", &log)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	raw, _ := os.ReadFile(c.FilePath())
	next := strings.Replace(string(raw), "report_interval: 40ms", "report_interval: 40ms\n  details:\n    enabled: true", 1)
	public, held := filepath.Join(r.result.Output, "public"), filepath.Join(r.result.Output, "held-public")
	if err = os.Rename(public, held); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := os.Stat(held); e == nil {
			os.Rename(held, public)
		}
	}()
	putRuntime(t, c.FilePath(), next)
	r.nextScan = time.Now().Add(time.Hour)
	r.nextReport = r.nextScan
	previous := r.original.FileDigest()
	r.step(context.Background(), time.Now())
	if r.config.Stats.Details.Enabled || r.original.FileDigest() != previous || log.Len() == 0 {
		t.Fatal("failed HTTP reconfigure committed runtime settings", log.String())
	}
	if err = os.Rename(held, public); err != nil {
		t.Fatal(err)
	}
	r.step(context.Background(), time.Now())
	if !r.config.Stats.Details.Enabled {
		t.Fatal("valid config did not recover")
	}
}

func TestScopedProxyReloadKeepsPublishedPage(t *testing.T) {
	c := runtimeFixture(t)
	r, err := startRuntime(context.Background(), c, true, "", "127.0.0.1:25646", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	raw, _ := os.ReadFile(c.FilePath())
	putRuntime(t, c.FilePath(), strings.Replace(string(raw), "server:\n", "server:\n  trusted_proxies: [\"fe80::1%eth0\"]\n", 1))
	original := r.result.Manifest.BuildID
	for n := 1; n <= 4; n++ {
		r.step(context.Background(), time.Now().Add(time.Duration(n)*time.Second))
		w := httptest.NewRecorder()
		r.handler.ServeHTTP(w, httptest.NewRequest("GET", "/posts/a/", nil))
		if w.Code != 200 || r.result.Manifest.BuildID != original || r.config.Server.TrustedProxies[0] != "127.0.0.1" {
			t.Fatal("bad proxy reload changed published state", w.Code)
		}
	}
}
