package entity

type PubsubMessage struct {
	From    string `json:"from"`
	Topic   string `json:"topic"`
	Kind    string `json:"kind"`
	Payload any    `json:"payload"`
}
