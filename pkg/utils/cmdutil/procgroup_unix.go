//go:build unix

package cmdutil

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/xhanio/errors"
)

func SetProcessGroup(p *exec.Cmd) {
	if p.SysProcAttr == nil {
		p.SysProcAttr = &syscall.SysProcAttr{}
	}
	p.SysProcAttr.Setpgid = true
}

// UseProcessGroup is for a command made with exec.CommandContext, as exec refuses its Cancel
// hook otherwise: canceling the context sends the whole group a SIGTERM.
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

// A group already gone is not an error.
func TerminateProcessGroup(p *exec.Cmd) error {
	return signalProcessGroup(p, syscall.SIGTERM)
}

// A group already gone is not an error.
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
