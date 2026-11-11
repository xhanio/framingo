package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/ioutil"
)

type cmd struct {
	ctx       context.Context
	bin       string
	args      []string
	envs      []string
	dir       string
	procGroup bool
	cancel    func(*os.Process) error
	waitDelay time.Duration
	maxBuffer int
	stdout    io.Writer
	stderr    io.Writer
	in        io.Reader
	outBuff   *bytes.Buffer
	errBuff   *bytes.Buffer
	p         *exec.Cmd
}

func New(bin string, args []string, opts ...Option) Command {
	return newCMD(bin, args, opts...)
}

func newCMD(bin string, args []string, opts ...Option) *cmd {
	c := &cmd{
		bin:     bin,
		args:    args,
		outBuff: &bytes.Buffer{},
		errBuff: &bytes.Buffer{},
	}
	c.applyOptions(opts...)
	c.setDefaults()
	p := exec.CommandContext(c.ctx, c.bin, c.args...)
	p.Env = os.Environ()
	p.Env = append(p.Env, c.envs...)
	p.Stdin = c.in
	if c.dir != "" {
		p.Dir = c.dir
	}
	if c.procGroup {
		UseProcessGroup(p)
	}
	if c.cancel != nil {
		cancel := c.cancel
		p.Cancel = func() error {
			return cancel(p.Process)
		}
	}
	if c.waitDelay > 0 {
		p.WaitDelay = c.waitDelay
	}
	p.Stdout = c.stream(c.stdout, c.outBuff)
	p.Stderr = c.stream(c.stderr, c.errBuff)
	c.p = p
	return c
}

// stream is where one of the command's outputs goes: its writer, or else its buffer.
func (c *cmd) stream(w io.Writer, buff *bytes.Buffer) io.Writer {
	switch {
	case w != nil:
		return w
	case c.maxBuffer > 0:
		return ioutil.NewLimitWriter(buff, c.maxBuffer)
	default:
		return buff
	}
}

func (c *cmd) String() string {
	return fmt.Sprintf("%s %s", c.bin, strings.Join(c.args, " "))
}

func (c *cmd) Output() string {
	return c.outBuff.String()
}

func (c *cmd) Error() string {
	return c.errBuff.String()
}

func (c *cmd) ExitCode() int {
	if c.p.ProcessState == nil {
		return -1
	}
	if c.p.ProcessState.Success() {
		return 0
	}
	return c.p.ProcessState.ExitCode()
}

func (c *cmd) Start() error {
	if err := c.p.Start(); err != nil {
		return errors.Wrapf(err, "failed to start %s", c)
	}
	return nil
}

func (c *cmd) Wait() error {
	if c.p.ProcessState != nil {
		return nil
	}
	return errors.Wrap(c.p.Wait())
}

func (c *cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *cmd) Terminate() error {
	if c.p.Process == nil {
		return errors.Conflict.Newf("%s has not started", c)
	}
	if c.procGroup {
		return TerminateProcessGroup(c.p)
	}
	return terminateProcess(c.p)
}

func (c *cmd) Kill() error {
	if c.p.Process == nil {
		return errors.Conflict.Newf("%s has not started", c)
	}
	if c.procGroup {
		return KillProcessGroup(c.p)
	}
	if err := c.p.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return errors.Wrapf(err, "failed to kill process %d", c.p.Process.Pid)
	}
	return nil
}
