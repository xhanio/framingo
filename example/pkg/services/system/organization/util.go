package organization

import (
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

func toEntity(o *orm.Organization) *entity.Organization {
	if o == nil {
		return nil
	}
	return &entity.Organization{
		ID:   o.ID,
		Name: o.Name,
	}
}
