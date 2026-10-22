package build

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
)

func buildFixture(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	put(t, root, "config.yaml", "site:\n  title: Test\n  base_url: https://example.com\npaths:\n  source: content\n  templates: templates\n  static: static\n  releases: runtime/releases\n  staging: runtime/staging\n  current: runtime/current\n")
	put(t, root, "content/posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\nBody.")
	put(t, root, "templates/page.html", `{{define "page"}}<!doctype html><html><head><title>{{.Page.Title}}</title></head><body>{{.Body}}{{range .Page.Items}}<a href="{{.URL}}">{{.Meta.Title}}</a>{{end}}</body></html>{{end}}`)
	put(t, root, "static/assets/theme.css", "body { color: black; }")
	collector, err := os.ReadFile("../../static/assets/stats.js")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "static/assets/stats.js", string(collector))
	put(t, root, "taxonomies.yaml", "schema_version: 1\n")
	c, err := config.Load(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestContentFingerprintAndVerifiedSnapshot(t *testing.T) {
	c := buildFixture(t)
	first, err := Scan(c, false)
	if err != nil {
		t.Fatal(err)
	}
	same, err := Scan(c, false)
	if err != nil || first != same {
		t.Fatal("nondeterministic scan")
	}
	file := filepath.Join(c.Paths.Source, "posts/a.md")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: B\nstatus: public\npublished: 2026-10-23\n---\nChanged.")
	if err = os.Chtimes(file, time.Now(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed, err := Scan(c, false)
	if err != nil || changed == first {
		t.Fatal("mtime-only scan")
	}
	if _, err = CreateSnapshot(c, t.TempDir(), first, false); err == nil {
		t.Fatal("accepted changed input during snapshot")
	}
	snapshot, err := CreateSnapshot(c, t.TempDir(), changed, false)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	put(t, c.Paths.Source, "posts/a.md", "invalid later edit")
	data, err := os.ReadFile(filepath.Join(snapshot.Config.Paths.Source, "posts/a.md"))
	if err != nil || string(data) == "invalid later edit" {
		t.Fatal("snapshot reads live source")
	}
	put(t, c.Paths.Static, "images/a.svg", "<svg/>")
	added, err := Scan(c, false)
	if err != nil || added == changed {
		t.Fatal("added input ignored")
	}
	if err = os.Remove(filepath.Join(c.Paths.Static, "images/a.svg")); err != nil {
		t.Fatal(err)
	}
	deleted, err := Scan(c, false)
	if err != nil || deleted == added {
		t.Fatal("removed input ignored")
	}
}

func TestOutputDoesNotParticipateInInputs(t *testing.T) {
	c := buildFixture(t)
	before, err := Scan(c, false)
	if err != nil {
		t.Fatal(err)
	}
	put(t, c.Paths.Releases, "r1/public/index.html", "output")
	after, err := Scan(c, false)
	if err != nil || before != after {
		t.Fatal("output participates in fingerprint")
	}
	preview, err := Scan(c, true)
	if err != nil || preview == after {
		t.Fatal("preview option missing from fingerprint")
	}
}

func TestConfigChangesRequireReload(t *testing.T) {
	c := buildFixture(t)
	put(t, filepath.Dir(c.FilePath()), "config.yaml", "site: {title: Changed, base_url: https://example.com}\n")
	if _, err := Scan(c, false); err == nil {
		t.Fatal("scan used stale parsed configuration")
	}
}

func TestSnapshotPreservesEmptyInputRoots(t *testing.T) {
	c := buildFixture(t)
	if err := os.RemoveAll(c.Paths.Static); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(c.Paths.Static, 0755); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := Scan(c, false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := CreateSnapshot(c, t.TempDir(), fingerprint, false)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if info, err := os.Stat(snapshot.Config.Paths.Static); err != nil || !info.IsDir() {
		t.Fatalf("empty root missing: %v", err)
	}
}
