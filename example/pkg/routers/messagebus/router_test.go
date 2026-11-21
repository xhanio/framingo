package messagebus

import (
	"testing"

	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/utils/apitest"
)

func TestRouter(t *testing.T) {
	apitest.CheckRouter(t, newRouter(nil, log.Default))
}
