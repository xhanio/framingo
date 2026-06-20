package rbac

import "github.com/xhanio/framingo/pkg/utils/sliceutil"

const (
	PermissionRoleRead         = "role_read"
	PermissionRoleWrite        = "role_write" //Admin_Only permission
	PermissionUserManageRead   = "local_user_manage_read"
	PermissionUserManageWrite  = "local_user_manage_write"
	PermissionUserRead         = "local_user_read"
	PermissionUserWrite        = "local_user_write"
	PermissionCertificateRead  = "certificate_read"
	PermissionCertificateWrite = "certificate_write"
)

var PermissionsAll = []string{
	PermissionRoleRead,
	PermissionRoleWrite,
	PermissionUserManageRead,
	PermissionUserManageWrite,
	PermissionUserRead,
	PermissionUserWrite,
	PermissionCertificateRead,
	PermissionCertificateWrite,
}

var PermissionsUser = []string{
	PermissionUserRead,
	PermissionUserWrite,
	PermissionCertificateRead,
	PermissionCertificateWrite,
}

var PermissionsReadonly = []string{
	PermissionUserRead,
	PermissionUserManageRead,
	PermissionCertificateRead,
}

func IsPermissionValid(permission string) bool {
	return sliceutil.In(permission, PermissionsAll...)
}
