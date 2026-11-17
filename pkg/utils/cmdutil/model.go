package cmdutil

// Command wraps an exec.Cmd and keeps its contract: a Command runs once, Wait
// is called once and never from two goroutines at the same time, and what the
// run produced - Output, Error, ExitCode - is read after Wait (or Run) has
// returned. Reading them while the command runs races the goroutines copying
// its output.
type Command interface {
	// Start starts the command and returns without waiting for it to end.
	Start() error
	// Wait waits for the started command to end and for its output to be
	// copied. Call it once: before Start it is an error, and so is a second
	// call - it does not repeat the first one's result.
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
	// no writer; WithMaxBuffer bounds them. They are complete, and safe to read, once Wait
	// has returned. To follow the output while the command runs, give the stream a writer
	// (WithStdout/WithStderr) instead: it is written from a goroutine of exec's, and two
	// different writers are written concurrently.
	Output() string
	Error() string
	// ExitCode is the exit code once Wait has returned, -1 before.
	ExitCode() int
	String() string
}
