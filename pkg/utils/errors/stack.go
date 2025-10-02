package errors

import (
	"runtime"

	"github.com/pkg/errors"
)

type stack []uintptr

const skip = 3 // skip runtime.Callers, callers(), and the error creation function (New/Wrap/etc)

// callers captures the current stack trace and returns it as a stack pointer
func callers() *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

// StackTrace converts the stack to a pkg/errors compatible StackTrace format
func (s *stack) StackTrace() errors.StackTrace {
	f := make([]errors.Frame, len(*s))
	for i := range f {
		f[i] = errors.Frame((*s)[i])
	}
	return f
}
