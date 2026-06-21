package entity

// PubsubMessage represents an event delivered through a subscription channel.
type PubsubMessage struct {
	From    string `json:"from"`
	Topic   string `json:"topic"`
	Kind    string `json:"kind"`
	Payload any    `json:"payload"`
}
