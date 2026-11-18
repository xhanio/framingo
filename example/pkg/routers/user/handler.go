package user

import (
	"net/http"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/entity"
)

// a role given to a user is checked against the role service here, keeping the two services apart

func (r *router) Create(c api.Context) error {
	var body api.UserCreateRequest
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
		// validated by the service
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
	userIDs := []int32{}
	if err := c.BindQuery().MustInt32s("ids", &userIDs).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := r.um.Delete(c, userIDs); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (r *router) Get(c api.Context) error {
	var userID int32
	if err := c.BindPath().MustInt32("id", &userID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	user, err := r.um.Get(c, userID)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (r *router) Update(c api.Context) error {
	var userID int32
	if err := c.BindPath().MustInt32("id", &userID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	var body api.UserUpdateRequest
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
	if opts.Role != nil {
		if *opts.Role == "" {
			return errors.InvalidArgument.Newf("role cannot be empty")
		}
		if _, err := r.rm.GetByName(c, *opts.Role); err != nil {
			return errors.Wrap(err)
		}
	}
	user, err := r.um.Update(c, userID, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (r *router) ResetPassword(c api.Context) error {
	var userID int32
	if err := c.BindPath().MustInt32("id", &userID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	var body api.UserResetPasswordRequest
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	opts := entity.UserResetPasswordOptions{
		Password: body.Password,
	}
	if err := r.um.ResetPassword(c, userID, opts); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusOK)
}

// only a local user has an own account
func currentUserID(c api.Context) (int32, error) {
	cred, ok := c.Credential()
	if !ok {
		return 0, errors.Unauthorized.Newf("no credential to act for")
	}
	userID, ok := cred.LocalUserID()
	if !ok {
		return 0, errors.Forbidden.Newf("a %s credential has no local user account", cred.Source)
	}
	return userID, nil
}

func (r *router) GetCurrent(c api.Context) error {
	userID, err := currentUserID(c)
	if err != nil {
		return errors.Wrap(err)
	}
	user, err := r.um.Get(c, userID)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (r *router) UpdateCurrent(c api.Context) error {
	userID, err := currentUserID(c)
	if err != nil {
		return errors.Wrap(err)
	}
	var body api.UserProfileUpdateRequest
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	opts := entity.UserUpdateOptions{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
		Title:     body.Title,
	}
	user, err := r.um.Update(c, userID, opts)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (r *router) ChangePassword(c api.Context) error {
	userID, err := currentUserID(c)
	if err != nil {
		return errors.Wrap(err)
	}
	var body api.UserChangePasswordRequest
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	opts := entity.UserChangePasswordOptions{
		OldPassword: body.OldPassword,
		Password:    body.Password,
	}
	if err := r.um.ChangePassword(c, userID, opts); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusOK)
}
