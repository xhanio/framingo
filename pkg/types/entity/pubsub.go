package entity

// PubsubMessage represents an event delivered through a subscription channel.
type PubsubMessage struct {
	From    string
	Topic   string
	Kind    string
	Payload any
}
