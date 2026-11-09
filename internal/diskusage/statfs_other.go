//go:build !unix

package diskusage

// Disk is not implemented here (Windows); the settings page shows "不明".
func Disk(string) (total, free uint64, ok bool) { return 0, 0, false }
