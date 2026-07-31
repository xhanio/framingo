package api

import (
	"time"
)

// RequestInfo is what the server's Info middleware resolves onto the request
// context for everything downstream. Route metadata declared in router.yaml -
// the permission, the poll flag - arrives flattened here; the parsed schema
// itself never leaves the server package.
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
