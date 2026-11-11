//go:build !unix

package cmdutil

import (
	"os"
	"os/exec"

	"github.com/xhanio/errors"
)

func SetProcessGroup(p *exec.Cmd) {}

func UseProcessGroup(p *exec.Cmd) {}

// TerminateProcessGroup has no gentler way than a kill without process groups.
func TerminateProcessGroup(p *exec.Cmd) error {
	return KillProcessGroup(p)
}

func KillProcessGroup(p *exec.Cmd) error {
	if err := p.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return errors.Wrapf(err, "failed to kill process %d", p.Process.Pid)
	}
	return nil
}

// terminateProcess has no gentler way than a kill without signals.
func terminateProcess(p *exec.Cmd) error {
	return KillProcessGroup(p)
}
