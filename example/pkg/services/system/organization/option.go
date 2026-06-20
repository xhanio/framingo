package organization

import "github.com/xhanio/framingo/pkg/utils/log"

type Option func(*manager)

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

type CreateOption struct {
	OrgIds          []int
	Username        string
	FirstName       string
	LastName        string
	Email           string
	Password        string
	ConfirmPassword string
	Title           string //Optional Param
	Roles           []string
}
