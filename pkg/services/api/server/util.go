package server

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap/zapcore"

	"github.com/xhanio/framingo/pkg/types/api"
)

// requestInfo builds the request record and reports whether a handler matched.
// The record is always returned - error paths log unmatched requests too - and
// the matched handler's route metadata is flattened onto it; the parsed schema
// itself stays inside this package.
func (s *server) requestInfo(c echo.Context) (*api.RequestInfo, bool) {
	r := c.Request()
	prevID := c.Request().Header.Get(api.HeaderKeyTrace)
	traceID := uuid.NewString()[:8]
	if prevID != "" {
		traceID = fmt.Sprintf("%s/%s", prevID, traceID)
	}
	c.Echo().Router().Find(r.Method, r.URL.EscapedPath(), c)
	s.log.Debugf("current call (endpoint %s) %s - %s", s.endpoint.Path, r.URL.Path, r.URL.EscapedPath())
	req := &api.RequestInfo{
		Server:    s.name,
		URI:       r.RequestURI,
		Method:    r.Method,
		Path:      r.URL.EscapedPath(),
		RawPath:   c.Path(),
		TraceID:   traceID,
		IP:        c.RealIP(),
		StartedAt: time.Now(),
	}
	// find the handler and group from this server instance
	key := s.requestKey(req)
	s.log.Debugf("looking for key %s", key.String())
	h, _ := s.matchHandler(key)
	if h == nil {
		s.log.Debugf("unable to locate api %s for req %s %s", key.String(), req.Method, req.RawPath)
		return req, false
	}
	req.Permission = h.Permission
	req.Poll = h.Poll
	return req, true
}

// requestKey builds the lookup key for a request, trimming the server's path
// prefix off the matched route path.
func (s *server) requestKey(req *api.RequestInfo) handlerKey {
	p := strings.TrimPrefix(req.RawPath, s.endpoint.Path)
	return handlerKey{
		Server: req.Server,
		Method: req.Method,
		Path:   path.Join("/", p),
	}
}

func (s *server) responseInfo(started time.Time, c echo.Context) *api.ResponseInfo {
	r := c.Response()
	resp := &api.ResponseInfo{
		Status: r.Status,
		Took:   time.Since(started).Round(time.Microsecond),
		Size:   uint64(r.Size),
	}
	if tid, ok := c.Get(api.ContextKeyTrace).(string); ok && tid != "" {
		resp.TraceID = tid
	}
	if ae, ok := c.Get(api.ContextKeyError).(*api.ErrorBody); ok {
		resp.Error = ae
		resp.Status = ae.Status
	}
	return resp
}

func (s *server) print(req *api.RequestInfo, resp *api.ResponseInfo) {
	parts := []any{
		fmt.Sprintf("%-15s", color.CyanString(req.Method)),
		fmt.Sprintf("%-21s", colorStatusCode(resp.Status)),
		fmt.Sprintf("%-25s", colorDuration(resp.Took)),
		fmt.Sprintf("%-22s", "size="+color.BlueString(humanize.Bytes(resp.Size))),
	}
	tid := resp.TraceID
	if tid == "" {
		tid = "n/a"
	}
	parts = append(parts, fmt.Sprintf("%-23s", "tid="+color.YellowString(tid)))
	parts = append(parts, req.URI)
	if resp.Status >= 400 && resp.Error != nil {
		if s.log.Level() == zapcore.DebugLevel {
			s.log.Errorf("%v", resp.Error.Origin)
		} else if resp.Status >= 500 {
			s.log.Errorf("%s", resp.Error.Origin)
		}
		s.log.Error(parts...)
	} else {
		s.log.Info(parts...)
	}
}

func colorStatusCode(statusCode int) string {
	str := color.RedString("%d", statusCode)
	if statusCode >= 200 && statusCode < 300 {
		str = color.GreenString("%d", statusCode)
	}
	return "status=" + str
}

func colorDuration(duration time.Duration) string {
	str := color.GreenString("%s", duration.Round(time.Nanosecond).String())
	if duration >= 5*time.Second {
		str = color.RedString("%s", duration.Round(time.Millisecond).String())
	} else if duration >= 1*time.Second {
		str = color.YellowString("%s", duration.Round(time.Microsecond).String())
	}
	return "took=" + str
}

// matchHandler looks up handler metadata by key with fallback:
// 1. exact key match
// 2. WS method fallback (WS routes registered as GET but keyed with WS)
// 3. ANY method match (same path, method=ANY)
// 4. wildcard path match — longest prefix wins, exact method over ANY
func (s *server) matchHandler(key handlerKey) (*handlerConfig, *handlerGroupConfig) {
	// exact match
	if h, ok := s.handlers[key]; ok {
		return h, s.groups[key]
	}
	// WS method fallback
	if key.Method == http.MethodGet {
		wsKey := handlerKey{Server: key.Server, Method: api.MethodWS, Path: key.Path}
		if h, ok := s.handlers[wsKey]; ok {
			return h, s.groups[wsKey]
		}
	}
	// ANY method fallback
	anyKey := handlerKey{Server: key.Server, Method: api.MethodAny, Path: key.Path}
	if h, ok := s.handlers[anyKey]; ok {
		return h, s.groups[anyKey]
	}
	// wildcard path match: iterate stored keys ending with /* and match
	// against the actual request path. Longest prefix wins (most specific),
	// and exact method takes priority over ANY at the same prefix length.
	reqPath := strings.TrimPrefix(key.Path, s.endpoint.Path)
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}

	var bestKey handlerKey
	var bestLen int
	var bestExact bool // true if matched via exact method (not ANY)

	for storedKey := range s.handlers {
		if !strings.HasSuffix(storedKey.Path, "/*") {
			continue
		}
		if storedKey.Server != key.Server {
			continue
		}
		isExact := storedKey.Method == key.Method
		isAny := storedKey.Method == api.MethodAny
		if !isExact && !isAny {
			continue
		}
		// "/api/*" → "/api/"
		wildcardBase := strings.TrimSuffix(storedKey.Path, "*")
		if !strings.HasPrefix(reqPath, wildcardBase) {
			continue
		}
		baseLen := len(wildcardBase)
		// prefer longer prefix; at same length prefer exact method over ANY
		if baseLen > bestLen || (baseLen == bestLen && isExact && !bestExact) {
			bestKey = storedKey
			bestLen = baseLen
			bestExact = isExact
		}
	}
	if bestLen > 0 {
		return s.handlers[bestKey], s.groups[bestKey]
	}
	return nil, nil
}
