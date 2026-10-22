package filelock

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLockChild(t *testing.T) {
	if path := os.Getenv("BLOGO_LOCK_CHILD"); path != "" {
		lock, err := Acquire(path)
		if err != nil {
			os.Exit(42)
		}
		lock.Close()
	}
}
func TestProcessExclusionAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target.lock")
	lock, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	child := func() error {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLockChild$")
		cmd.Env = append(os.Environ(), "BLOGO_LOCK_CHILD="+path)
		return cmd.Run()
	}
	err = child()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 42 {
		t.Fatalf("second process acquired lock: %v", err)
	}
	if err = lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err = child(); err != nil {
		t.Fatalf("lock did not release: %v", err)
	}
}
