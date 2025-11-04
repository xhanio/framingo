package server

import (
	"time"

	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

func (m *manager) ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed || err == nil {
		return
	}
	req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
	if !ok || req == nil {
		req = m.requestInfo(c)
	}
	resp, ok := c.Get(common.ContextKeyAPIResponseInfo).(*api.ResponseInfo)
	if !ok || resp == nil {
		ae := api.WrapError(err, c)
		resp = &api.ResponseInfo{
			Status: ae.Status,
			Error:  ae,
			Took:   time.Since(req.StartedAt).Round(time.Microsecond),
		}
		m.print(req, resp)
	}
	if err := c.JSON(resp.Status, resp.Error); err != nil {
		m.log.Errorf("failed to send json response: %v", err)
	}
}
