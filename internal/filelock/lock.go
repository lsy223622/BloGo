// Package filelock provides nonblocking process locks for publication and reports.
package filelock

import (
	"fmt"
	"os"
)

type Lock struct{ file *os.File }

func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("target is locked (%s): %w", path, err)
	}
	return &Lock{file: f}, nil
}

// Closing releases the native lock. The file remains to preserve its inode for
// contenders; unlinking a lock file could create two independently locked files.
func (l *Lock) Close() error { return l.file.Close() }
