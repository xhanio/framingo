package b

// an option outside option.go is not this rule's: the convention is the file's
type Option func()

func WithAnything() Option { return nil }
