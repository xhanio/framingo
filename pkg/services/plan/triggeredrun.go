package plan

import (
	"sync/atomic"

	"github.com/xhanio/framingo/pkg/types/entity"
)

// triggeredRun goes with a firing of Trigger: all its runs take the source, only the first
// the ID Trigger returned, as retries and requeues are runs of their own.
type triggeredRun struct {
	id      string
	source  entity.PlanRunSource
	claimed atomic.Bool
}
