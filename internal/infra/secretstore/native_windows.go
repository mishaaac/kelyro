//go:build windows

package secretstore

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/mishaaac/kelyro/internal/storage"
)

const (
	credentialTypeGeneric      = 1
	credentialPersistLocalMach = 2
	windowsErrorNotFound       = syscall.Errno(1168)
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type windowsCredentialManager struct{}

func newNativeBackend() (nativeBackend, error) {
	for _, procedure := range []*syscall.LazyProc{procCredWrite, procCredRead, procCredDelete, procCredFree} {
		if err := procedure.Find(); err != nil {
			return nil, fmt.Errorf("Windows Credential Manager API is unavailable: %w", err)
		}
	}
	return windowsCredentialManager{}, nil
}

func (windowsCredentialManager) Get(name string) (string, error) {
	target, err := syscall.UTF16PtrFromString(windowsTarget(name))
	if err != nil {
		return "", fmt.Errorf("encode credential target: %w", err)
	}
	var credential *windowsCredential
	result, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(target)),
		credentialTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	if result == 0 {
		return "", windowsCredentialError("read", callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credential)))
	if credential == nil || credential.CredentialBlobSize == 0 || credential.CredentialBlob == nil {
		return "", storage.ErrSecretNotFound
	}
	value := unsafe.Slice(credential.CredentialBlob, int(credential.CredentialBlobSize))
	return string(value), nil
}

func (windowsCredentialManager) Set(name, value string) error {
	target, err := syscall.UTF16PtrFromString(windowsTarget(name))
	if err != nil {
		return fmt.Errorf("encode credential target: %w", err)
	}
	username, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return fmt.Errorf("encode credential username: %w", err)
	}
	blob := []byte(value)
	credential := windowsCredential{
		Type:               credentialTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credentialPersistLocalMach,
		UserName:           username,
	}
	result, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&credential)), 0)
	for index := range blob {
		blob[index] = 0
	}
	if result == 0 {
		return windowsCredentialError("store", callErr)
	}
	return nil
}

func (windowsCredentialManager) Delete(name string) error {
	target, err := syscall.UTF16PtrFromString(windowsTarget(name))
	if err != nil {
		return fmt.Errorf("encode credential target: %w", err)
	}
	result, _, callErr := procCredDelete.Call(
		uintptr(unsafe.Pointer(target)),
		credentialTypeGeneric,
		0,
	)
	if result == 0 {
		return windowsCredentialError("delete", callErr)
	}
	return nil
}

func windowsTarget(name string) string {
	return serviceName + ":" + name
}

func windowsCredentialError(operation string, err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno == windowsErrorNotFound {
		return storage.ErrSecretNotFound
	}
	return fmt.Errorf("Windows Credential Manager could not %s the credential: %w", operation, err)
}

var _ nativeBackend = windowsCredentialManager{}
