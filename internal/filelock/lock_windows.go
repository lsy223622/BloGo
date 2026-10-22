package filelock

import (
	"os"
	"syscall"
	"unsafe"
)

func lockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	ok, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx").Call(file.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		return err
	}
	return nil
}
