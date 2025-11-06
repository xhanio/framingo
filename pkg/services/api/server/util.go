package server

import (
	"fmt"
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
	c.Echo().Router().Find(r.Method, r.URL.EscapedPath(), c)
	// find the handler and group from this server instance
	key := req.Key(s.endpoint.Path)
	req.Handler = s.handlers[key]
	req.HandlerGroup = s.groups[key]
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
