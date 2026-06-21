package role

import (
	"net/http"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

func (r *router) Create(c api.Context) error {
	var opt entity.RoleCreateOptions
	if err := c.Bind(&opt); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := r.rm.Create(c, opt); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusCreated)
}

func (r *router) List(c api.Context) error {
	roles, err := r.rm.List(c)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, roles)
}

func (r *router) Delete(c api.Context) error {
	var roleID int32
	if err := c.BindPath().MustInt32("id", &roleID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if roleID == rbac.RoleAdminID {
		return errors.BadRequest.Newf("admin role cannot be deleted")
	}
	if err := r.rm.Delete(c, roleID); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (r *router) Update(c api.Context) error {
	var roleID int32
	if err := c.BindPath().MustInt32("id", &roleID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if roleID == rbac.RoleAdminID {
		return errors.BadRequest.Newf("admin role cannot be modified")
	}
	var opt entity.RoleUpdateOptions
	if err := c.Bind(&opt); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := r.rm.Update(c, roleID, opt); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusOK)
}

func (r *router) ListPermissions(c api.Context) error {
	var roleID int32
	if err := c.BindPath().MustInt32("id", &roleID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if roleID == rbac.RoleAdminID {
		return c.JSON(http.StatusOK, rbac.PermissionsAll)
	}
	permissions, err := r.rm.GetPermissions(c, roleID)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, permissions)
}

func (r *router) SetPermissions(c api.Context) error {
	var roleID int32
	if err := c.BindPath().MustInt32("id", &roleID).BindError(); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if roleID == rbac.RoleAdminID {
		return errors.BadRequest.Newf("admin role already has all permissions")
	}
	var opts entity.PermissionSetOptions
	if err := c.Bind(&opts); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	for _, p := range opts.Permissions {
		if !rbac.IsPermissionValid(p) {
			return errors.BadRequest.Newf("invalid permission: %s", p)
		}
	}
	if err := r.rm.SetPermissions(c, roleID, opts); err != nil {
		return errors.Wrap(err)
	}
	return c.NoContent(http.StatusOK)
}

func (r *router) Handlers() map[string]any {
	return api.DiscoverHandlers(r)
}
