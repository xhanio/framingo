package server

import (
	"time"

	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

func (s *server) errorHandler(err error, c echo.Context) {
	if c.Response().Committed || err == nil {
		return
	}
	req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
	if !ok || req == nil {
		// The record is wanted even when no handler matched, so the 404
		// itself gets logged with the request's facts.
		req, _ = s.requestInfo(c)
	}
	resp, ok := c.Get(common.ContextKeyAPIResponseInfo).(*api.ResponseInfo)
	if !ok || resp == nil {
		ae := api.WrapError(err, c)
		ae.Source = s.Name()
		resp = &api.ResponseInfo{
			Status: ae.Status,
			Error:  ae,
			Took:   time.Since(req.StartedAt).Round(time.Microsecond),
		}
		s.print(req, resp)
	}
	if err := c.JSON(resp.Status, resp.Error); err != nil {
		s.log.Errorf("failed to send json response: %v", err)
	}
}
