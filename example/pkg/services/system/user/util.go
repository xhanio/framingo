package user

import (
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

func toEntity(user *orm.User, contact *orm.Contact) *entity.User {
	if user == nil {
		return nil
	}
	if contact == nil {
		contact = &orm.Contact{}
	}
	return &entity.User{
		ID:             user.ID,
		Username:       user.Username,
		Title:          contact.Title,
		FirstName:      contact.FirstName,
		LastName:       contact.LastName,
		Email:          contact.Email,
		Role:           user.Role,
		ChangePassword: user.Expired,
	}
}
