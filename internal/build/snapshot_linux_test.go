package build

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestScanRejectsLinksAndSpecialFiles(t *testing.T) {
	c := buildFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(c.Paths.Static, "linked.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(c, false); err == nil {
		t.Fatal("accepted outside link")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(c.Paths.Static, "pipe.txt")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(c, false); err == nil {
		t.Fatal("accepted special file")
	}
}
