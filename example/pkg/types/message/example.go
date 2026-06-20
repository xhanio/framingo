package message

type Example struct {
	From    string
	To      string
	Message string
}

func (Example) Kind() string {
	return "example_event"
}
