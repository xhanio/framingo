package api

import (
	"time"
)

// RequestInfo is set on the request context by the server's Info middleware.
type RequestInfo struct {
	Server     string
	URI        string
	Method     string
	Path       string
	RawPath    string
	TraceID    string
	IP         string
	StartedAt  time.Time
	Permission string
	Poll       bool
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
