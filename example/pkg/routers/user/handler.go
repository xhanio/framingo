package user

import (
	"net/http"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

func (r *router) Create(c api.Context) error {
	credential, ok := c.Credential()
	if !ok || credential == nil {
		return errors.Unauthorized.New()
	}
	var body api.UserCreateBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	if _, err := r.rm.GetByName(c, body.Role); err != nil {
		return errors.Wrap(err)
	}
	opts := entity.UserCreateOptions{
		Username:       body.Username,
		FirstName:      body.FirstName,
		LastName:       body.LastName,
		Email:          body.Email,
		Password:       body.Password,
		Title:          body.Title,
		Role:           body.Role,
		ChangePassword: body.ChangePassword,
	}
	user, err := r.um.Create(c, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusCreated, api.UserCreateResponse{UserID: user.ID})
}

func (r *router) List(c api.Context) error {
	opts := entity.UserListOptions{}
	if sortBy := c.QueryParam("sort_by"); sortBy != "" {
		opts.SortBy = sortBy
		if c.QueryParam("sort_order") == "desc" {
			opts.Desc = true
		}
	}
	users, err := r.um.List(c, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	if users == nil {
		users = []*entity.User{}
	}
	return c.JSON(http.StatusOK, users)
}

func (r *router) Delete(c api.Context) error {
	credential, ok := c.Credential()
	if !ok || credential == nil {
		return errors.Unauthorized.New()
	}
	userIDs := []int32{}
	if err := c.BindQuery().MustInt32s("ids", &userIDs).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	for _, id := range userIDs {
		if id == credential.UserID {
			return errors.BadRequest.Newf("cannot delete your own user account")
		}
	}
	if err := r.um.Delete(c, userIDs); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (r *router) Get(c api.Context) error {
	credential, ok := c.Credential()
	if !ok || credential == nil {
		return errors.Unauthorized.New()
	}
	var userID int32
	if err := c.BindPath().MustInt32("id", &userID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	perms, err := r.rm.GetPermissionsByName(c, credential.Role)
	if err != nil {
		return errors.Wrap(err)
	}
	if !sliceutil.In(rbac.PermissionUserManageRead, perms...) && credential.UserID != userID {
		return errors.Forbidden.Newf("no access to other user's information")
	}
	user, err := r.um.Get(c, userID)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (r *router) Update(c api.Context) error {
	credential, ok := c.Credential()
	if !ok || credential == nil {
		return errors.Unauthorized.New()
	}
	var userID int32
	if err := c.BindPath().MustInt32("id", &userID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if credential.Role != rbac.RoleAdmin && credential.UserID != userID {
		return errors.Forbidden.Newf("no access to other user's information")
	}
	var body api.UserUpdateBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	opts := entity.UserUpdateOptions{
		Username:  body.Username,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
		Title:     body.Title,
		Role:      body.Role,
		Expired:   body.ChangePassword,
	}
	if credential.UserID == userID {
		if opts.Username != nil && *opts.Username != credential.UserName {
			return errors.Forbidden.Newf("cannot update your own username")
		}
		if opts.Role != nil && *opts.Role != credential.Role {
			return errors.Forbidden.Newf("cannot update your own role")
		}
	}
	if opts.Role != nil {
		if *opts.Role == "" {
			return errors.InvalidArgument.Newf("role cannot be empty")
		}
		if _, err := r.rm.GetByName(c, *opts.Role); err != nil {
			return errors.Wrap(err)
		}
	}
	before, err := r.um.Get(c, userID)
	if err != nil {
		return errors.Wrap(err)
	}
	if opts.Username != nil && *opts.Username != before.Username {
		cred := &entity.Credential{
			Source:           preset.AuthSourceLocalUser,
			UserName:         before.Username,
			OrganizationName: preset.DefaultOrganizationName,
		}
		if r.am.HasSession(c, cred) {
			return errors.BadRequest.Newf("cannot update username: user has alive sessions")
		}
	}
	user, err := r.um.Update(c, userID, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	if opts.Role != nil && *opts.Role != before.Role {
		r.am.Logout(c, &entity.Credential{
			Source:           preset.AuthSourceLocalUser,
			UserName:         user.Username,
			OrganizationName: preset.DefaultOrganizationName,
		})
	}
	return c.JSON(http.StatusOK, user)
}

func (r *router) ResetPassword(c api.Context) error {
	credential, ok := c.Credential()
	if !ok || credential == nil {
		return errors.Unauthorized.New()
	}
	var userID int32
	if err := c.BindPath().MustInt32("id", &userID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	isOwn := credential.UserID == userID
	if credential.Role != rbac.RoleAdmin && !isOwn {
		return errors.Forbidden.Newf("no access to other user's information")
	}
	var body api.UserResetPasswordBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	opts := entity.UserResetPasswordOptions{
		Password:    body.Password,
		OldPassword: body.OldPassword,
	}
	if err := r.um.ResetPassword(c, isOwn, userID, opts); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusOK)
}

func (r *router) Handlers() map[string]any {
	return api.DiscoverHandlers(r)
}
