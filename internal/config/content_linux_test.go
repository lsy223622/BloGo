package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataRootCannotContainInputsThroughSymlink(t *testing.T) {
	file := fixture(t, "")
	root := filepath.Dir(file)
	if err := os.Symlink(root, filepath.Join(root, "linked-data")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("paths:\n  data: linked-data\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(file)
	if err == nil {
		err = c.Validate("build", filepath.Join(root, "out"))
	}
	if err == nil {
		t.Fatal("data root can contain content through a symlink")
	}
}
