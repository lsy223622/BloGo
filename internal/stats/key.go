package stats

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lsy223622/BloGo/internal/filelock"
)

func VisitorDigest(key []byte, day, visitor string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(day))
	mac.Write([]byte{0})
	mac.Write([]byte(visitor))
	return mac.Sum(nil)
}

func LoadOrCreateKey(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := filelock.Acquire(path + ".lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
			return nil, fmt.Errorf("visitor key must be an ordinary private file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(data) != 32 {
			return nil, fmt.Errorf("visitor key must contain exactly 32 bytes")
		}
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if _, err = file.Write(key); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return key, nil
}
