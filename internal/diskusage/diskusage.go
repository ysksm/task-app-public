// Package diskusage reports the size of a directory tree and the space on
// the file system that holds it (the capacity section of the settings page).
package diskusage

import (
	"io/fs"
	"os"
	"path/filepath"
)

// DirBytes is the total size of the regular files under dir (0 if missing).
func DirBytes(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// FileBytes is the size of a file (0 if missing).
func FileBytes(path string) int64 {
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}
