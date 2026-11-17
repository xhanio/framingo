package repository

import (
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

// Repository is the one instance behind every repo contract. Its Name,
// Dependencies and Transaction come with each contract, through the
// repo.Repository they all embed.
type Repository interface {
	repo.User
	repo.Organization
	repo.Role
	repo.Certificate
	repo.HelloWorld
}
