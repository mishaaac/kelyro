//go:build windows

package cli

import (
	"os"
	"syscall"
)

const enableEchoInput = 0x0004

var (
	kernel32Console    = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleMode = kernel32Console.NewProc("SetConsoleMode")
)

func disableEcho(input *os.File) (func() error, error) {
	info, err := input.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return func() error { return nil }, nil
	}

	handle := syscall.Handle(input.Fd())
	var original uint32
	if err := syscall.GetConsoleMode(handle, &original); err != nil {
		return nil, err
	}
	if err := setConsoleMode(handle, original&^enableEchoInput); err != nil {
		return nil, err
	}
	return func() error { return setConsoleMode(handle, original) }, nil
}

func setConsoleMode(handle syscall.Handle, mode uint32) error {
	result, _, callErr := procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
	if result == 0 {
		return callErr
	}
	return nil
}
