//go:build !windows

package updatecache

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
