package api

import "time"

const (
	EventWidgetByMonth = "month"
	EventWidgetByDay   = "day"
	EventWidgetByHour  = "hour"
)

type EventWidgetRequest struct {
	Period    int
	StartTime time.Time
	EndTime   time.Time
	Frequency string
}

type EventWidgetResponse struct {
	Time        time.Time `json:"timestamp"`
	CountSeen   int       `json:"readCount"`
	CountUnseen int       `json:"unreadCount"`
}
