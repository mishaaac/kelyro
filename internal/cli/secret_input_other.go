//go:build !linux && !darwin && !windows

package cli

import "os"

func disableEcho(*os.File) (func() error, error) {
	return func() error { return nil }, nil
}
