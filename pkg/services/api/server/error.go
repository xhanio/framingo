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
		req = s.requestInfo(c)
	}
	resp, ok := c.Get(common.ContextKeyAPIResponseInfo).(*api.ResponseInfo)
	if !ok || resp == nil {
		ae := api.WrapError(err, c)
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
