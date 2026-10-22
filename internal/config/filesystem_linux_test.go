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
