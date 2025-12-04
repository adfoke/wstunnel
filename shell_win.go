//go:build windows

package main

import (
	"os/exec"
)

func startShell(stream *WSStream) error {
	cmd := exec.Command("cmd.exe")
	cmd.Stdin = stream
	cmd.Stdout = stream
	cmd.Stderr = stream
	return cmd.Run()
}
