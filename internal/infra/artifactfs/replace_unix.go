//go:build !windows

package artifactfs

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
