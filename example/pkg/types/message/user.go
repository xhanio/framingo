package message

// Published once the change is committed; users are named by ID, unique across organizations and rename-proof.

type LocalUsersDeleted struct {
	UserIDs []int32
}

func (LocalUsersDeleted) Kind() string {
	return "local_users_deleted_event"
}

type LocalUserPasswordReset struct {
	UserID int32
}

func (LocalUserPasswordReset) Kind() string {
	return "local_user_password_reset_event"
}

// LocalUserIdentityChanged: credentials issued before it no longer describe the user.
type LocalUserIdentityChanged struct {
	UserID int32
}

func (LocalUserIdentityChanged) Kind() string {
	return "local_user_identity_changed_event"
}
