package driver

import (
	"encoding/json"
	"strings"
)

const channelBufferSize = 256

// subscriber holds a subscriber's name and delivery channel.
type subscriber struct {
	name string
	ch   chan Message
}

type eventMessage struct {
	Publisher string          `json:"publisher"`
	Topic     string          `json:"topic"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
}

// topicMatches checks if a subscription topic matches a publish topic.
// "app" matches "app", "app/module", "app/module/component".
func topicMatches(subTopic, eventTopic string) bool {
	if subTopic == eventTopic {
		return true
	}
	return strings.HasPrefix(eventTopic, subTopic+"/")
}
