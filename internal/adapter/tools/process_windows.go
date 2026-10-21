//go:build windows

package tools

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func prepareCommand(cmd *exec.Cmd) (func() error, func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = windows.CloseHandle(job) }
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		cleanup()
		return nil, nil, err
	}
	// Associate the suspended process before it can create any descendants.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW, HideWindow: true}
	cmd.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	started := func() error {
		process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err != nil {
			return err
		}
		defer func() { _ = windows.CloseHandle(process) }()
		if err = windows.AssignProcessToJobObject(job, process); err != nil {
			return err
		}
		snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
		if err != nil {
			return err
		}
		defer func() { _ = windows.CloseHandle(snapshot) }()
		entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
		err = windows.Thread32First(snapshot, &entry)
		for n := 0; err == nil && n < 100000; n++ {
			if entry.OwnerProcessID == uint32(cmd.Process.Pid) {
				thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
				if err != nil {
					return err
				}
				defer func() { _ = windows.CloseHandle(thread) }()
				_, err = windows.ResumeThread(thread)
				return err
			}
			err = windows.Thread32Next(snapshot, &entry)
		}
		return errors.New("cannot find suspended tool thread")
	}
	return started, cleanup, nil
}
