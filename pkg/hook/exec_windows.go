//go:build windows

package hook

import "os/exec"

// setProcGroup is a no-op on Windows: syscall.SysProcAttr has no
// Setpgid field there, and hooks (bash test.sh scripts) aren't expected
// to run on Windows anyway. This exists purely so the package builds
// cross-platform.
func setProcGroup(_ *exec.Cmd) {}

// killProcessGroup falls back to killing just the immediate process,
// since POSIX process groups don't exist on Windows.
func killProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
