//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// Vault is a console executable. Launch it without a second visible Windows
// console window while retaining its stdout/stderr pipes and exit status.
// CREATE_NO_WINDOW (0x08000000) is a process-creation presentation flag only;
// it has no effect on the Vault recovery engine.
func prepareVaultProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
}
