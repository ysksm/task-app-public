//go:build unix

package diskusage

import "syscall"

// Disk returns the size of the file system holding path and the space
// available to unprivileged users. ok is false when it cannot be read.
func Disk(path string) (total, free uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	return uint64(st.Blocks) * uint64(st.Bsize), uint64(st.Bavail) * uint64(st.Bsize), true
}
