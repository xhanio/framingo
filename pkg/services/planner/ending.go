package planner

import "github.com/xhanio/framingo/pkg/types/entity"

type ending struct {
	plan  *entity.Plan
	ended <-chan struct{}
}
