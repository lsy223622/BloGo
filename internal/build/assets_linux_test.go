package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContentAssetsRejectSymlinks(t *testing.T) {
	c := buildFixture(t)
	target := filepath.Join(t.TempDir(), "private.png")
	if err := os.WriteFile(target, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(c.Paths.Assets, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(c.Paths.Assets, "leak.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(c, false); err == nil {
		t.Fatal("asset symlink accepted")
	}
}
