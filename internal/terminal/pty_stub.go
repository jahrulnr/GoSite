//go:build windows

package terminal

import (
	"errors"
	"os"
	"os/exec"
)

// ptyHandle is a stub used on Windows. The real pty package only supports
// Unix-like systems; this stub lets `go build` succeed on Windows for
// non-runtime tests and documentation tooling.
type ptyHandle struct {
	master *os.File
}

func ptyStart(_ *exec.Cmd, _, _ int) (ptyHandle, error) {
	return ptyHandle{}, errors.New("pty is not supported on Windows")
}

func ptyResize(_ *os.File, _, _ int) error {
	return errors.New("pty is not supported on Windows")
}

func ptyKill(_ int) error {
	return errors.New("pty is not supported on Windows")
}
