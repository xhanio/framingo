//go:build unix

package cmdutil

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/xhanio/errors"
)

func UseProcessGroup(p *exec.Cmd) {
	if p.SysProcAttr == nil {
		p.SysProcAttr = &syscall.SysProcAttr{}
	}
	p.SysProcAttr.Setpgid = true
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
