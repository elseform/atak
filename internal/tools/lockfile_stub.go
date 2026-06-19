//go:build !linux

package tools

func WriteLock(pid int) error { return nil }
func ClearLock() error        { return nil }
func CheckStaleLock()         {}
