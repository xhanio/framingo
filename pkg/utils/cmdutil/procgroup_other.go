//go:build !unix

package cmdutil

import "os/exec"

func UseProcessGroup(p *exec.Cmd) {}
