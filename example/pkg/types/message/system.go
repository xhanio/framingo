package message

import "time"

type SystemTimezoneChanged struct {
	From *time.Location
	To   *time.Location
}

func (SystemTimezoneChanged) Kind() string {
	return "system_timezone_changed_event"
}

type SystemHostnameChanged struct {
	From string
	To   string
}

func (SystemHostnameChanged) Kind() string {
	return "system_hostname_changed_event"
}
