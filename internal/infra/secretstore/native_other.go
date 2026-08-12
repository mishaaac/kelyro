//go:build !linux && !darwin && !windows

package secretstore

import "fmt"

func newNativeBackend() (nativeBackend, error) {
	return nil, fmt.Errorf("no native credential adapter is available for this operating system")
}
