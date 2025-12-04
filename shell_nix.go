//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func startShell(stream *WSStream) error {
	// Use bash if available, otherwise sh
	shell := "/bin/bash"
	if _, err := os.Stat(shell); os.IsNotExist(err) {
		shell = "/bin/sh"
	}

	c := exec.Command(shell)

	// Start the command with a pty
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	// Copy from WSStream (remote input) to PTY (shell input)
	go func() { _, _ = io.Copy(ptmx, stream) }()
	
	// Copy from PTY (shell output) to WSStream (remote output)
	_, _ = io.Copy(stream, ptmx)

	return c.Wait()
}
