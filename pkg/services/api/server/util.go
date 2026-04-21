package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap/zapcore"

	"github.com/xhanio/framingo/pkg/types/api"
)

func (s *server) requestInfo(c echo.Context) *api.RequestInfo {
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
	key := req.Key(s.endpoint.Path)
	s.log.Debugf("looking for key %s", key)
	h, g := s.matchHandler(key, req.Method, req.Path)
	if h != nil && g != nil {
		req.Handler = h
		req.HandlerGroup = g
	} else {
		s.log.Debugf("unable to locate api %s for req %s %s", key, req.Method, req.RawPath)
	}
	return req
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
// 2. ANY method match (same path, method=ANY)
// 3. wildcard path match — longest prefix wins, exact method over ANY
func (s *server) matchHandler(key, method, reqPath string) (*api.Handler, *api.HandlerGroup) {
	// exact match
	if h, ok := s.handlers[key]; ok {
		return h, s.groups[key]
	}
	// ANY method fallback
	anyKey := strings.Replace(key, "<"+method+">", "<"+api.MethodAny+">", 1)
	if h, ok := s.handlers[anyKey]; ok {
		return h, s.groups[anyKey]
	}
	// wildcard path match: iterate stored keys ending with /* and match
	// against the actual request path. Longest prefix wins (most specific),
	// and exact method takes priority over ANY at the same prefix length.
	reqPath = strings.TrimPrefix(reqPath, s.endpoint.Path)
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}
	serverMethod := fmt.Sprintf("%s<%s>", s.name, method)
	serverAny := fmt.Sprintf("%s<%s>", s.name, api.MethodAny)

	var bestKey string
	var bestLen int
	var bestExact bool // true if matched via exact method (not ANY)

	for storedKey := range s.handlers {
		if !strings.HasSuffix(storedKey, "/*") {
			continue
		}
		isExact := strings.HasPrefix(storedKey, serverMethod)
		isAny := strings.HasPrefix(storedKey, serverAny)
		if !isExact && !isAny {
			continue
		}
		// "http<ANY>/api/*" → "/api/"
		wildcardBase := strings.TrimSuffix(storedKey[strings.Index(storedKey, ">")+1:], "*")
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
	if bestKey != "" {
		return s.handlers[bestKey], s.groups[bestKey]
	}
	return nil, nil
}
