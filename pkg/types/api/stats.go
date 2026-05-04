package api

import (
	"path"
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

func (s *RequestInfo) Key(prefix string) HandlerKey {
	p := strings.TrimPrefix(s.RawPath, prefix)
	p = path.Join("/", p)
	return HandlerKey{
		Server: s.Server,
		Method: s.Method,
		Path:   p,
	}
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
