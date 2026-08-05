//go:build unix

package hook

import (
	"os/exec"
	"syscall"
)

// setProcGroup configures cmd to run in its own process group, so that
// killProcessGroup can later terminate the whole tree of descendants
// spawned by the hook (e.g. a backgrounded/forgotten child process), not
// just the immediate bash process.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to cmd's entire process group.
func killProcessGroup(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
