package message

type DeleteLocalUsers struct {
	Usernames []string
}

func (e DeleteLocalUsers) Kind() string {
	return "delete_local_users_event"
}

type ResetLocalUserPassword struct {
	Username string
}

func (e ResetLocalUserPassword) Kind() string {
	return "reset_local_user_password_event"
}
