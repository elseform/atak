//go:build linux

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
	return "/tmp/stalker-tex.lock"
}

func WriteLock(pid int) error {
	return os.WriteFile(lockPath(), []byte(fmt.Sprintf("%d\n", pid)), 0600)
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
		log.Printf("stalker-tex: killed orphan process group %d from previous crash", pid)
	}
	_ = os.Remove(lockPath())
}
