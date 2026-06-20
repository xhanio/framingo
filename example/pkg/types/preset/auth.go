package preset

import "time"

var (
	AuthSourceAPIToken  = "api-token"
	AuthSourceAgent     = "agent"
	AuthSourceLocalUser = "local-user"
	AuthSourceLdapUser  = "ldap-user"

	SessionCookieMaxAge = 24 * time.Hour
	SessionExpiration   = 6 * time.Hour
)
