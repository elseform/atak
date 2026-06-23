//go:build !windows

package tools

import "os/exec"

// JobHandle is a no-op on non-Windows platforms.
type JobHandle struct{}

func NewJob() (JobHandle, error)         { return JobHandle{}, nil }
func AssignJob(_ JobHandle, _ *exec.Cmd) {}
func CloseJob(_ JobHandle)               {}
