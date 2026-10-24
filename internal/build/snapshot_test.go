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
	put(t, root, "config.yaml", "server:\n  base_url: https://example.com\n")
	put(t, root, "content/posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\nBody.")
	put(t, root, "templates/page.html", `{{define "page"}}<!doctype html><html><head><title>{{.Page.Title}}</title></head><body>{{.Body}}{{range .Page.Items}}<a href="{{.URL}}">{{.Meta.Title}}</a>{{end}}</body></html>{{end}}`)
	put(t, root, "static/assets/theme.css", "body { color: black; }")
	collector, err := os.ReadFile("../../static/assets/stats.js")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "static/assets/stats.js", string(collector))
	put(t, root, "content/site.yaml", "schema_version: 1\ntitle: Test\n")
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

func TestContentSettingsAndAssetsSnapshot(t *testing.T) {
	c := buildFixture(t)
	first, err := Scan(c, false)
	if err != nil {
		t.Fatal(err)
	}
	put(t, c.Paths.Source, "site.yaml", "schema_version: 1\ntitle: Changed\n")
	changed, err := Scan(c, false)
	if err != nil || changed == first {
		t.Fatal("settings omitted from fingerprint", err)
	}
	put(t, c.Paths.Source, "assets/photo.png", "image bytes")
	image, err := Scan(c, false)
	if err != nil || image == changed {
		t.Fatal("attachment omitted from fingerprint", err)
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\n![Photo](/media/photo.png)")
	built, err := Build(c, filepath.Join(t.TempDir(), "site"), false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(built.Output, "public/media/photo.png"))
	if err != nil || string(data) != "image bytes" {
		t.Fatal("attachment mapping", err)
	}
	put(t, c.Paths.Static, "media/photo.png", "conflict")
	if _, err = Build(c, filepath.Join(t.TempDir(), "conflict"), false); err == nil {
		t.Fatal("static/media collision accepted")
	}
	if err = os.Remove(filepath.Join(c.Paths.Static, "media/photo.png")); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(c.Paths.Assets, "photo.png")); err != nil {
		t.Fatal(err)
	}
	if _, err = Build(c, filepath.Join(t.TempDir(), "missing"), false); err == nil {
		t.Fatal("missing attachment accepted")
	}
}

func TestConfigChangesRequireReload(t *testing.T) {
	c := buildFixture(t)
	put(t, filepath.Dir(c.FilePath()), "config.yaml", "server: {base_url: https://changed.example}\n")
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
