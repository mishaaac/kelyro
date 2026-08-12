//go:build linux

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

func disableEcho(input *os.File) (func() error, error) {
	info, err := input.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return func() error { return nil }, nil
	}

	var original syscall.Termios
	if err := ioctl(input.Fd(), syscall.TCGETS, &original); err != nil {
		return nil, err
	}
	updated := original
	updated.Lflag &^= syscall.ECHO
	if err := ioctl(input.Fd(), syscall.TCSETS, &updated); err != nil {
		return nil, err
	}
	return func() error { return ioctl(input.Fd(), syscall.TCSETS, &original) }, nil
}

func ioctl(fd uintptr, request uintptr, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(state)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
