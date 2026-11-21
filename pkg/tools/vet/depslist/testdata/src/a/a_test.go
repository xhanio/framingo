package a

// a test's double embeds what it fakes, and lists nothing
type double struct {
	Sender
}

func (d *double) Name() string            { return "double" }
func (d *double) Dependencies() []Service { return nil }
