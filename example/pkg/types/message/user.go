package message

// The user service publishes these once the change is committed. They name
// users by ID: it survives a rename and is unique across organizations, which
// a username is not.

// LocalUsersDeleted: the local user accounts with these IDs no longer exist.
type LocalUsersDeleted struct {
	UserIDs []int32
}

func (LocalUsersDeleted) Kind() string {
	return "local_users_deleted_event"
}

// LocalUserPasswordReset: the user's password was replaced.
type LocalUserPasswordReset struct {
	UserID int32
}

func (LocalUserPasswordReset) Kind() string {
	return "local_user_password_reset_event"
}

// LocalUserIdentityChanged: the username or role a credential is built from
// changed, so a credential issued before the change no longer describes the
// user.
type LocalUserIdentityChanged struct {
	UserID int32
}

func (LocalUserIdentityChanged) Kind() string {
	return "local_user_identity_changed_event"
}
