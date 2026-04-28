package cmdutil

import (
	"bufio"
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

const ErrorCodeCmd string = "CMD_ERROR"

type cmd struct {
	ctx        context.Context
	bin        string
	args       []string
	envs       []string
	async      bool
	dir        string
	cancel     func(*os.Process) error
	waitDelay  time.Duration
	maxBuffer  int
	stdout     io.Writer
	stderr     io.Writer
	in         io.Reader
	out        io.Reader
	outBuff    *bytes.Buffer
	errBuff    *bytes.Buffer
	p          *exec.Cmd
}

func New(bin string, args []string, opts ...Option) Command {
	return newCMD(bin, args, opts...)
}

func newCMD(bin string, args []string, opts ...Option) *cmd {
	c := &cmd{
		bin:  bin,
		args: args,
	}
	c.apply(opts...)
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	p := exec.CommandContext(c.ctx, c.bin, c.args...)
	p.Env = os.Environ()
	p.Env = append(p.Env, c.envs...)
	p.Stdin = c.in
	if c.dir != "" {
		p.Dir = c.dir
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
	if c.maxBuffer > 0 {
		c.outBuff = bytes.NewBuffer(make([]byte, 0, c.maxBuffer))
		c.errBuff = bytes.NewBuffer(make([]byte, 0, c.maxBuffer))
	} else {
		c.outBuff = bytes.NewBuffer([]byte{})
		c.errBuff = bytes.NewBuffer([]byte{})
	}
	var outWriter io.Writer = c.outBuff
	var errWriter io.Writer = c.errBuff
	if c.maxBuffer > 0 {
		outWriter = ioutil.NewLimitWriter(c.outBuff, c.maxBuffer)
		errWriter = ioutil.NewLimitWriter(c.errBuff, c.maxBuffer)
	}
	if c.stdout != nil {
		outWriter = io.MultiWriter(outWriter, c.stdout)
	}
	if c.stderr != nil {
		errWriter = io.MultiWriter(errWriter, c.stderr)
	}
	if c.async {
		pr, pw := io.Pipe()
		c.out = pr
		p.Stdout = pw
	} else {
		p.Stdout = outWriter
	}
	p.Stderr = errWriter
	c.p = p
	return c
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

func (c *cmd) Print(fns ...PrintFunc) {
	scanner := bufio.NewScanner(c.out)
	scanner.Split(bufio.ScanLines)
	go func() {
		for {
			if !scanner.Scan() {
				return
			}
			line := scanner.Text()
			for _, fn := range fns {
				fn(line)
			}
			_, err := c.outBuff.WriteString(line)
			if err != nil {
				return
			}
		}
	}()
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
	err := c.p.Start()
	if err != nil {
		return errors.New(
			errors.WithMessage("(%s) %s", err, c.Error()),
			errors.WithCode(ErrorCodeCmd, map[string]string{
				"cmd":    c.String(),
				"stage":  "start",
				"stdout": c.Output(),
				"stderr": c.Error(),
			}),
		)
	}
	if c.async {
		return nil
	}
	return c.Wait()
}

func (c *cmd) Wait() error {
	if c.p.ProcessState == nil {
		err := c.p.Wait()
		if err != nil {
			return errors.New(
				errors.WithMessage("(%s) %s", err, c.Error()),
				errors.WithCode(ErrorCodeCmd, map[string]string{
					"cmd":    c.String(),
					"stage":  "wait",
					"stdout": c.Output(),
					"stderr": c.Error(),
				}),
			)
		}
	}
	return nil
}
