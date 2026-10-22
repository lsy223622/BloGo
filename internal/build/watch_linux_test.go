package build

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestWatchStateSkipsUnchangedAndRecovers(t *testing.T) {
	c := buildFixture(t)
	var log bytes.Buffer
	w := newWatcher(c, &log)
	first, err := w.step()
	if err != nil || first == nil {
		t.Fatalf("initial: %v", err)
	}
	if result, err := w.step(); err != nil || result != nil {
		t.Fatalf("unchanged rebuilt: %v", err)
	}
	restarted := newWatcher(c, &log)
	if result, err := restarted.step(); err != nil || result != nil {
		t.Fatal("restart forgot current fingerprint")
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\n---\nBroken.")
	if _, err = w.step(); err == nil {
		t.Fatal("invalid source published")
	}
	before := log.Len()
	if result, err := w.step(); err != nil || result != nil || log.Len() != before {
		t.Fatal("same failed fingerprint retried or relogged")
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\nCorrected.")
	if result, err := w.step(); err != nil || result == nil {
		t.Fatalf("source change did not recover: %v", err)
	}
	if !strings.Contains(log.String(), "Published") {
		t.Fatal("publication not logged")
	}
}

func TestWatchRetriesTransientIO(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged process")
	}
	c := buildFixture(t)
	var log bytes.Buffer
	w := newWatcher(c, &log)
	if _, err := w.step(); err != nil {
		t.Fatal(err)
	}
	put(t, c.Paths.Source, "posts/a.md", "---\ntitle: A\nstatus: public\npublished: 2026-10-23\n---\nChanged.")
	if err := os.Chmod(c.Paths.Staging, 0500); err != nil {
		t.Fatal(err)
	}
	if _, err := w.step(); err == nil {
		t.Fatal("expected transient write failure")
	}
	if err := os.Chmod(c.Paths.Staging, 0755); err != nil {
		t.Fatal(err)
	}
	if result, err := w.step(); err != nil || result == nil {
		t.Fatalf("transient failure suppressed retry: %v", err)
	}
}
