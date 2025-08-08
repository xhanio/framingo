package cmd

type PrintFunc func(line string)

type Command interface {
	Output() string
	Error() string
	Start() error
	Wait() error
	Print(fns ...PrintFunc)
	String() string
	ExitCode() int
}
