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

	"github.com/xhanio/framingo/pkg/utils/errors"
)

const ErrorCodeCmd string = "CMD_ERROR"

type cmd struct {
	ctx     context.Context
	bin     string
	args    []string
	envs    []string
	async   bool
	in      io.Reader
	out     io.Reader
	outBuff *bytes.Buffer
	errBuff *bytes.Buffer
	p       *exec.Cmd
}

func New(bin string, args []string, opts ...Option) Command {
	return newCMD(bin, args, opts...)
}

func newCMD(bin string, args []string, opts ...Option) *cmd {
	c := &cmd{
		bin:  bin,
		args: args,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	p := exec.CommandContext(c.ctx, c.bin, c.args...)
	p.Env = os.Environ()
	p.Env = append(p.Env, c.envs...)
	p.Stdin = c.in
	c.outBuff = bytes.NewBuffer([]byte{})
	c.errBuff = bytes.NewBuffer([]byte{})
	if c.async {
		pr, pw := io.Pipe()
		c.out = pr
		p.Stdout = pw
	} else {
		p.Stdout = c.outBuff
	}
	p.Stderr = c.errBuff
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
