//go:build windows

package tools

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// JobHandle wraps a Windows Job Object handle.
// Closing the handle with KILL_ON_JOB_CLOSE set kills all assigned processes,
// which provides automatic cleanup if the Go process exits unexpectedly.
type JobHandle struct{ h windows.Handle }

// NewJob creates a Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
func NewJob() (JobHandle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return JobHandle{}, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return JobHandle{}, err
	}
	return JobHandle{h: job}, nil
}

// AssignJob assigns cmd's process to the job object.
// Must be called after cmd.Start().
func AssignJob(j JobHandle, cmd *exec.Cmd) {
	if j.h == 0 || cmd.Process == nil {
		return
	}
	proc, err := windows.OpenProcess(
		windows.PROCESS_ALL_ACCESS, false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		return
	}
	_ = windows.AssignProcessToJobObject(j.h, proc)
	_ = windows.CloseHandle(proc)
}

// CloseJob closes the job handle. With KILL_ON_JOB_CLOSE, closing the handle
// kills all assigned processes if they are still running.
func CloseJob(j JobHandle) {
	if j.h != 0 {
		_ = windows.CloseHandle(j.h)
	}
}

func SetProcAttr(cmd *exec.Cmd) {}

func KillProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
