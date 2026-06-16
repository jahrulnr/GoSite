//go:build !windows

package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// ptyHandle is the real PTY master used on Linux/Unix. We keep the file
// descriptor wrapped so the rest of the package can be tested with the stub
// implementation on non-Unix platforms.
type ptyHandle struct {
	master *os.File
}

// ptyStart spawns cmd with a freshly allocated PTY and returns the master fd.
func ptyStart(cmd *exec.Cmd, cols, rows int) (ptyHandle, error) {
	if cmd == nil {
		return ptyHandle{}, errors.New("nil command")
	}

	// Allocate PTY with the requested initial size. The shell will receive the
	// SIGWINCH-equivalent resize events via ptyResize below.
	master, tty, err := pty.Open()
	if err != nil {
		return ptyHandle{}, fmt.Errorf("pty.Open: %w", err)
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if err := pty.Setsize(master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols), X: uint16(cols) * 8, Y: uint16(rows) * 16}); err != nil {
		_ = master.Close()
		_ = tty.Close()
		return ptyHandle{}, fmt.Errorf("pty.Setsize: %w", err)
	}

	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	// Ensure the child has a controlling TTY (required for bash job control).
	// Also put it into its own process group so Kill can signal the entire
	// group including any backgrounded long-running processes (e.g. tail -f).
	//
	// Without Setsid+Setctty bash prints:
	//   "cannot set terminal process group ... no job control in this shell"
	// because it is not a session leader with a controlling terminal.
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attr.Setsid = true
	attr.Setctty = true
	// Ctty is the fd number *in the child* that should become controlling TTY.
	// Since we wire `tty` into cmd.Stdin, the child will have it as fd 0.
	attr.Ctty = 0
	// Note: do NOT also Setpgid here. With Setsid=true, the child is already
	// placed into a fresh process group (pgid == pid). Setting both can trip
	// EPERM in restricted environments and is redundant for our kill-by-pgid
	// strategy (ptyKill uses -pid).
	cmd.SysProcAttr = attr

	if err := cmd.Start(); err != nil {
		_ = master.Close()
		_ = tty.Close()
		return ptyHandle{}, fmt.Errorf("cmd.Start: %w", err)
	}
	// tty is duplicated onto the child stdin/stdout/stderr so we can close the
	// caller-side copy after start without breaking the child.
	_ = tty.Close()

	return ptyHandle{master: master}, nil
}

// ptyResize updates the winsize of the master fd.
func ptyResize(master *os.File, cols, rows int) error {
	if master == nil {
		return errors.New("nil master")
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	return pty.Setsize(master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols), X: uint16(cols) * 8, Y: uint16(rows) * 16})
}

// ptyKill sends SIGKILL to the process group identified by pid. We use the
// negative pid convention so the signal targets the entire group, ensuring
// any tail-f style background processes started by the shell are also
// terminated.
func ptyKill(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	return syscall.Kill(-pid, syscall.SIGKILL)
}
