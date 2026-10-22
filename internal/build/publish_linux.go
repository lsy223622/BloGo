package build

import (
	"os"
	"path/filepath"
)

func publicationSupported() error { return nil }
func atomicCurrent(current, release, id string) error {
	temporary := filepath.Join(filepath.Dir(current), ".current-"+id)
	if err := os.Symlink(release, temporary); err != nil {
		return err
	}
	defer os.Remove(temporary)
	return os.Rename(temporary, current)
}
