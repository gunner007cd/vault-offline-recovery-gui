//go:build !windows

package main

import "os/exec"

func prepareVaultProcess(_ *exec.Cmd) {}
