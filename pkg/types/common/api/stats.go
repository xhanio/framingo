package api

import (
	"fmt"
	"time"
)

type RequestInfo struct {
	URI          string
	Method       string
	Path         string
	RawPath      string
	TraceID      string
	IP           string
	StartedAt    time.Time
	Handler      *Handler
	HandlerGroup *HandlerGroup
}

func (s *RequestInfo) Key() string {
	return fmt.Sprintf("<%s>%s", s.Method, s.RawPath)
}

type ResponseInfo struct {
	Status  int
	Took    time.Duration
	Size    uint64
	TraceID string
	Error   *ErrorBody
}

type Stats struct {
	RequestInfo
	ResponseInfo
}
