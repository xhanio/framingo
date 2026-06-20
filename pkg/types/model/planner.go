package model

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Planner interface {
	common.Service
	Add(todo *entity.Plan) error
	Cancel(id string) error
	Delete(id string, force bool) error
	GetResult(id string) (any, error)
	Stats(opts entity.PlannerStatsOptions) ([]*entity.PlannerStats, error)
}
