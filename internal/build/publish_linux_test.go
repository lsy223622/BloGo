package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lsy223622/BloGo/internal/filelock"
)

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
