package config

import (
	"os"
	"path/filepath"
)

func existingAncestor(path string) (string, error) {
	p, err := resolved(path)
	if err != nil {
		return "", err
	}
	for {
		_, err = os.Stat(p)
		if err == nil {
			return p, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		p = filepath.Dir(p)
	}
}
