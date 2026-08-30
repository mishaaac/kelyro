//go:build !windows

package researchcachefs

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
