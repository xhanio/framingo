//go:build unix

package cmdutil

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/xhanio/errors"
)

// SetProcessGroup starts p in a process group of its own, for its owner to signal with
// TerminateProcessGroup and KillProcessGroup.
func SetProcessGroup(p *exec.Cmd) {
	if p.SysProcAttr == nil {
		p.SysProcAttr = &syscall.SysProcAttr{}
	}
	p.SysProcAttr.Setpgid = true
}

// UseProcessGroup is SetProcessGroup for a command made with exec.CommandContext: canceling
// the context sends the whole group a SIGTERM.
func UseProcessGroup(p *exec.Cmd) {
	SetProcessGroup(p)
	p.Cancel = func() error {
		pgid := p.Process.Pid
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				// exec reports the exit status instead of a cancel error
				return os.ErrProcessDone
			}
			return errors.Wrapf(err, "failed to signal process group %d", pgid)
		}
		return nil
	}
}

// TerminateProcessGroup sends a SIGTERM to the group of a started command. A group already
// gone is not an error.
func TerminateProcessGroup(p *exec.Cmd) error {
	return signalProcessGroup(p, syscall.SIGTERM)
}

// KillProcessGroup sends a SIGKILL to the group of a started command, which ends what
// ignored the SIGTERM. A group already gone is not an error.
func KillProcessGroup(p *exec.Cmd) error {
	return signalProcessGroup(p, syscall.SIGKILL)
}

func signalProcessGroup(p *exec.Cmd, sig syscall.Signal) error {
	pgid := p.Process.Pid
	if err := syscall.Kill(-pgid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return errors.Wrapf(err, "failed to signal process group %d", pgid)
	}
	return nil
}
