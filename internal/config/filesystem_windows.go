package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func sameFilesystem(paths ...string) error {
	var volume string
	for i, p := range paths {
		p, err := existingAncestor(p)
		if err != nil {
			return err
		}
		ptr, err := syscall.UTF16PtrFromString(p)
		if err != nil {
			return err
		}
		buf := make([]uint16, 32768)
		ret, _, callErr := syscall.NewLazyDLL("kernel32.dll").NewProc("GetVolumePathNameW").Call(uintptr(unsafe.Pointer(ptr)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if ret == 0 {
			return fmt.Errorf("volume for %s: %w", p, callErr)
		}
		v := strings.ToLower(filepath.Clean(syscall.UTF16ToString(buf)))
		if i == 0 {
			volume = v
		} else if volume != v {
			return fmt.Errorf("staging, releases and current must share a filesystem")
		}
	}
	return nil
}

// Unix mode bits do not describe Windows ACLs. Production key permission
// checks run on Linux, the deployment platform.
func privateKey(string) error { return nil }

func writableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: expected output directory", path)
	}
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := syscall.CreateFile(ptr, syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return syscall.CloseHandle(h)
}
