package cmdutil

// Command runs once; call Wait once, from one goroutine, and read Output, Error and ExitCode
// only after it returns.
type Command interface {
	Start() error
	// Wait errors before Start and on a second call.
	Wait() error
	Run() error
	// Terminate sends SIGTERM (to the group under WithProcessGroup), or kills where signals are
	// unsupported; an ended command is not an error.
	Terminate() error
	Kill() error
	// Output and Error capture only streams given no writer (bounded by WithMaxBuffer); custom
	// writers are called from exec's goroutines, stdout and stderr concurrently.
	Output() string
	Error() string
	// ExitCode is -1 until Wait returns.
	ExitCode() int
	String() string
}
