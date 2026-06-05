package model

import (
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Planner interface {
	Add(todo *entity.PlannerTODO) error
	Cancel(id string) error
	Delete(id string, force bool) error
	GetResult(id string) (any, error)
	Stats(opts entity.PlannerStatsOptions) ([]*entity.PlannerStats, error)
}
