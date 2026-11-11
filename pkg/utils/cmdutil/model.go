package cmdutil

type Command interface {
	// Start starts the command and returns without waiting for it to end.
	Start() error
	// Wait waits for the started command to end.
	Wait() error
	// Run starts the command and waits for it to end.
	Run() error
	// Terminate sends the command a SIGTERM, to its whole process group with
	// WithProcessGroup; where there are no signals it kills it. A command that already ended
	// is not an error.
	Terminate() error
	// Kill sends the command, or its process group, a SIGKILL.
	Kill() error
	// Output and Error hold what the command wrote to stdout and stderr, for a stream given
	// no writer; WithMaxBuffer bounds them.
	Output() string
	Error() string
	ExitCode() int
	String() string
}
