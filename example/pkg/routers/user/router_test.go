package user

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/utils/apitest"
)

func TestRouter(t *testing.T) {
	prefixes := map[string]bool{}
	for _, rt := range apitest.CheckRouter(t, newRouter(nil, nil, log.Default)) {
		prefixes[rt.Prefix] = true
	}
	assert.Equal(t, map[string]bool{"/users": true, "/user": true}, prefixes, "both groups")
}
