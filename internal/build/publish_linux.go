package build

import (
	"os"
	"path/filepath"
)

func publicationSupported() error { return nil }
func atomicCurrent(current, release, id string) error {
	temporary := filepath.Join(filepath.Dir(current), ".current-"+id)
	target, err := filepath.Rel(filepath.Dir(current), release)
	if err != nil {
		return err
	}
	if err := os.Symlink(target, temporary); err != nil {
		return err
	}
	defer os.Remove(temporary)
	return os.Rename(temporary, current)
}
