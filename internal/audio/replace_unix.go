//go:build !windows

package audio

import "os"

func replaceFile(source, target string) error {
	// Convert stages beside the target, so rename replaces the directory
	// entry on the same filesystem without first deleting an existing output.
	return os.Rename(source, target)
}
