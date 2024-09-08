//go:build windows
// +build windows

package terraform

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

var procAttrs = &windows.SysProcAttr{
	CreationFlags: windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NEW_PROCESS_GROUP,
}

func interrupt(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	dll, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer dll.Release()
	f, err := dll.FindProc("AttachConsole")
	if err != nil {
		return err
	}
	r1, _, err := f.Call(uintptr(pid))
	if r1 == 0 && err != syscall.ERROR_ACCESS_DENIED {
		return err
	}
	f, err = dll.FindProc("SetConsoleCtrlHandler")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(0, 1)
	if r1 == 0 {
		return err
	}
	f, err = dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(windows.CTRL_BREAK_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	r1, _, err = f.Call(windows.CTRL_C_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	return nil
}
