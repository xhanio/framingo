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

// the record is returned even when no handler matched, for error-path logging
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
		var origin error = resp.Error
		if resp.Error.Origin != nil { // a body a handler returned as is has none
			origin = resp.Error.Origin
		}
		switch {
		case s.log.Level() == zapcore.DebugLevel, resp.Status == http.StatusInternalServerError:
			// the server's own fault: code, details and the stack that locates it
			s.log.Errorf("%+v", origin)
		case resp.Status >= http.StatusInternalServerError:
			// a dependency down or a deadline passed: a stack adds nothing, and an outage repeats it on every request
			s.log.Errorf("%s", origin)
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

// fallback order: exact, WS (registered as GET), ANY, then longest wildcard prefix with exact method over ANY
func (s *server) matchHandler(key handlerKey) (*handlerConfig, *handlerGroupConfig) {
	if h, ok := s.handlers[key]; ok {
		return h, s.groups[key]
	}
	if key.Method == http.MethodGet {
		wsKey := handlerKey{Server: key.Server, Method: api.MethodWS, Path: key.Path}
		if h, ok := s.handlers[wsKey]; ok {
			return h, s.groups[wsKey]
		}
	}
	anyKey := handlerKey{Server: key.Server, Method: api.MethodAny, Path: key.Path}
	if h, ok := s.handlers[anyKey]; ok {
		return h, s.groups[anyKey]
	}
	reqPath := strings.TrimPrefix(key.Path, s.endpoint.Path)
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}

	var bestKey handlerKey
	var bestLen int
	var bestExact bool

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
		wildcardBase := strings.TrimSuffix(storedKey.Path, "*")
		if !strings.HasPrefix(reqPath, wildcardBase) {
			continue
		}
		baseLen := len(wildcardBase)
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
