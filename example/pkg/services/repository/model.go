package repository

import (
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

// Repository is the one instance behind every repo contract.
type Repository interface {
	repo.User
	repo.Organization
	repo.Role
	repo.Certificate
	repo.HelloWorld
}
