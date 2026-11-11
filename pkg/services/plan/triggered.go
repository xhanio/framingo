package plan

import (
	"sync/atomic"
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
)

// triggered goes with a firing of Trigger: all its runs take the source, only the first
// the ID Trigger returned, as retries and requeues are runs of their own. If no run claims
// the ID, the Cancel, Delete or Stop that dropped the firing does.
type triggered struct {
	id      string
	source  entity.PlanRunSource
	at      time.Time
	claimed atomic.Bool
}

func (tr *triggered) claim() bool {
	return tr.claimed.CompareAndSwap(false, true)
}
