package config

import (
	"fmt"
	"os"
	"syscall"
)

func sameFilesystem(paths ...string) error {
	var device uint64
	for i, p := range paths {
		p, err := existingAncestor(p)
		if err != nil {
			return err
		}
		var st syscall.Stat_t
		if err = syscall.Stat(p, &st); err != nil {
			return err
		}
		if i == 0 {
			device = uint64(st.Dev)
		} else if uint64(st.Dev) != device {
			return fmt.Errorf("staging, releases and current must share a filesystem")
		}
	}
	return nil
}

func privateKey(path string) error {
	i, err := os.Stat(path)
	if err != nil {
		return err
	}
	if i.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("key permissions must exclude group and other users")
	}
	return nil
}

func writableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: expected output directory", path)
	}
	return syscall.Access(path, 3)
}
