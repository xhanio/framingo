package api

import (
	"fmt"
	"strings"
	"time"
)

type RequestInfo struct {
	Server       string
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

func (s *RequestInfo) Key(prefix string) string {
	path := strings.TrimPrefix(s.RawPath, prefix)
	return fmt.Sprintf("%s<%s>%s", s.Server, s.Method, path)
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
