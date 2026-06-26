//go:build linux || darwin

package tools

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func lockPath() string {
	return "/tmp/atak.lock"
}

func WriteLock(pid int) error {
	return os.WriteFile(lockPath(), fmt.Appendf(nil, "%d\n", pid), 0600)
}

func ClearLock() error {
	return os.Remove(lockPath())
}

func CheckStaleLock() {
	data, err := os.ReadFile(lockPath())
	if err != nil {
		return
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		_ = os.Remove(lockPath())
		return
	}
	if syscall.Kill(pid, 0) == nil {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		log.Printf("atak: killed orphan process group %d from previous crash", pid)
	}
	_ = os.Remove(lockPath())
}
