package build

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/filelock"
)

func TestPublishedDataMovesWithUpgrade(t *testing.T) {
	c := buildFixture(t)
	if _, err := Publish(c); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "upgraded")
	if err := os.Rename(filepath.Dir(c.FilePath()), moved); err != nil {
		t.Fatal(err)
	}
	next, err := config.Load(filepath.Join(moved, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err = next.Validate("publish", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadPublished(next.Paths.Current); err != nil {
		t.Fatal(err)
	}
}

func TestPublishAssetsBelongToCurrentSnapshot(t *testing.T) {
	c := buildFixture(t)
	c.Build.KeepReleases = 2
	put(t, c.Paths.Templates, "page.html", `{{define "page"}}<html><head><link rel="stylesheet" href="{{index .Assets "theme.css"}}"></head><body>{{.Body}}</body></html>{{end}}`)
	first, err := Publish(c)
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(first.Output, "public/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	asset := regexp.MustCompile(`assets/theme\.[0-9a-f]{16}\.css`).Find(html)
	if len(asset) == 0 {
		t.Fatal("missing old reference")
	}
	put(t, c.Paths.Static, "assets/theme.css", "body{color:blue}")
	second, err := Publish(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(second.Output, "public", string(asset))); !os.IsNotExist(err) {
		t.Fatal("old asset inherited into current snapshot")
	}
	if _, err = os.Stat(filepath.Join(first.Output, "public", string(asset))); err != nil {
		t.Fatal("complete rollback release lost", err)
	}
	if err = os.Remove(filepath.Join(c.Paths.Static, "assets/stats.js")); err != nil {
		t.Fatal(err)
	}
	if _, err = Publish(c); err == nil {
		t.Fatal("missing collector published")
	}
	current, err := filepath.EvalSymlinks(c.Paths.Current)
	if err != nil || current != second.Output {
		t.Fatal("missing collector switched current")
	}
}

func TestDamagedHistoryDoesNotBlockHealthyPublication(t *testing.T) {
	c := buildFixture(t)
	old, err := Publish(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(old.Output, "meta/manifest.json"), []byte("broken history"), 0640); err != nil {
		t.Fatal(err)
	}
	put(t, c.Paths.Static, "assets/theme.css", "body{color:blue}")
	newRelease, err := Publish(c)
	if err != nil {
		t.Fatal("healthy publication blocked by history", err)
	}
	current, err := filepath.EvalSymlinks(c.Paths.Current)
	if err != nil || current != newRelease.Output {
		t.Fatal("new current not selected", err)
	}
	if _, err = VerifyRelease(c.Paths.Current); err != nil {
		t.Fatal(err)
	}
}

func TestPublishAtomicVersionAndRetention(t *testing.T) {
	c := buildFixture(t)
	c.Build.KeepReleases = 2
	first, err := Publish(c)
	if err != nil {
		t.Fatal(err)
	}
	target, err := filepath.EvalSymlinks(c.Paths.Current)
	if err != nil || filepath.Base(target) != first.Manifest.BuildID {
		t.Fatalf("current %s %v", target, err)
	}
	if _, err = VerifyRelease(target); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		path string
		mode os.FileMode
	}{{target, 0755}, {filepath.Join(target, "meta"), 0750}, {filepath.Join(target, "meta/manifest.json"), 0640}} {
		info, err := os.Stat(check.path)
		if err != nil || info.Mode().Perm() != check.mode {
			t.Fatalf("release permission %s: %v %v", check.path, info, err)
		}
	}
	lock, err := filelock.Acquire(c.Paths.Current + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Publish(c); err == nil {
		t.Fatal("concurrent publish accepted")
	}
	lock.Close()
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: Broken\nstatus: public\n---\n")
	if _, err = Publish(c); err == nil {
		t.Fatal("invalid content published")
	}
	unchanged, err := filepath.EvalSymlinks(c.Paths.Current)
	if err != nil || unchanged != target {
		t.Fatal("failed publish switched current")
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\nNew.")
	for n := 0; n < 3; n++ {
		if _, err = Publish(c); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(c.Paths.Releases)
	if err != nil || len(entries) != 2 {
		t.Fatalf("retention %d: %v", len(entries), err)
	}
	if _, err = VerifyRelease(c.Paths.Current); err != nil {
		t.Fatal(err)
	}
	current, err := filepath.EvalSymlinks(c.Paths.Current)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		candidate := filepath.Join(c.Paths.Releases, entry.Name())
		if candidate != current {
			if err = atomicCurrent(c.Paths.Current, candidate, "rollback"); err != nil {
				t.Fatal(err)
			}
			manifest, err := VerifyRelease(c.Paths.Current)
			if err != nil || manifest.BuildID != entry.Name() {
				t.Fatalf("rollback mismatch %v", err)
			}
			break
		}
	}
}

func TestFailedCurrentSwitchPreservesLink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged process")
	}
	c := buildFixture(t)
	if _, err := Publish(c); err != nil {
		t.Fatal(err)
	}
	before, err := os.Readlink(c.Paths.Current)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(c.Paths.Current)
	if err = os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0700)
	if err = atomicCurrent(c.Paths.Current, "/unreachable-release", "failed-switch"); err == nil {
		t.Fatal("switch unexpectedly succeeded")
	}
	after, err := os.Readlink(c.Paths.Current)
	if err != nil || before != after {
		t.Fatal("failed switch changed current")
	}
}
