package config

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLinuxFilesystemAndKeyPermissions(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "key")
	if err := os.WriteFile(key, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := privateKey(key); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(key, 0644); err != nil {
		t.Fatal(err)
	}
	if err := privateKey(key); err == nil {
		t.Fatal("accepted public key permissions")
	}
	var a, b syscall.Stat_t
	if syscall.Stat(dir, &a) != nil || syscall.Stat("/dev/shm", &b) != nil || a.Dev == b.Dev {
		t.Skip("second filesystem unavailable")
	}
	if err := sameFilesystem(dir, "/dev/shm", dir); err == nil {
		t.Fatal("accepted cross-filesystem publication")
	}
}

func TestLinkedOutputOverlap(t *testing.T) {
	c, err := Load(fixture(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(c.Paths.Source), "linked-output")
	if err = os.Symlink(c.Paths.Source, link); err != nil {
		t.Fatal(err)
	}
	if err = c.Validate("build", filepath.Join(link, "out")); err == nil {
		t.Fatal("accepted symlink-concealed input/output overlap")
	}
}

func TestCommandIsolationFromUnreadablePaths(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged process")
	}
	c, err := Load(fixture(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(c.Paths.Source)
	blocked := filepath.Join(base, "blocked")
	if err = os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blocked, 0700)
	c.Stats.DatabasePath = filepath.Join(blocked, "db")
	c.Stats.VisitorKeyPath = filepath.Join(blocked, "key")
	c.Stats.ManifestPath = filepath.Join(blocked, "manifest.json")
	if err = c.Validate("build", filepath.Join(base, "out")); err != nil {
		t.Fatalf("build probed private stats files: %v", err)
	}
	for _, name := range []string{"manifest.json", "db", "key"} {
		if err = os.WriteFile(filepath.Join(base, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c.Stats.ManifestPath = filepath.Join(base, "manifest.json")
	c.Stats.DatabasePath = filepath.Join(base, "db")
	c.Stats.VisitorKeyPath = filepath.Join(base, "key")
	c.Stats.PublicOutput = filepath.Join(base, "stats-public/stats.json")
	c.Stats.PrivateOutput = filepath.Join(base, "private/report.html")
	c.Paths.Source = filepath.Join(blocked, "source")
	c.Paths.Templates = filepath.Join(blocked, "templates")
	c.Paths.Static = filepath.Join(blocked, "static")
	c.Paths.Taxonomies = filepath.Join(blocked, "taxonomies.yaml")
	for _, cmd := range []string{"stats", "report"} {
		if cmd == "report" {
			c.Stats.VisitorKeyPath = filepath.Join(blocked, "key")
		}
		if err = c.Validate(cmd, ""); err != nil {
			t.Fatalf("%s probed content: %v", cmd, err)
		}
	}
}

func TestCurrentLinkInsideReleases(t *testing.T) {
	c, err := Load(fixture(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(c.Paths.Source)
	c.Paths.Staging = filepath.Join(base, "staging")
	c.Paths.Releases = filepath.Join(base, "releases")
	c.Paths.Current = filepath.Join(base, "current")
	release := filepath.Join(c.Paths.Releases, "r1")
	if err = os.MkdirAll(release, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(release, c.Paths.Current); err != nil {
		t.Fatal(err)
	}
	c.Stats.ManifestPath = filepath.Join(c.Paths.Current, "meta/manifest.json")
	if err = c.Validate("publish", ""); err != nil {
		t.Fatal(err)
	}
}
