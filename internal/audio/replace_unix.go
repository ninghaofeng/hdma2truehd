//go:build !windows

package audio

import "os"

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
