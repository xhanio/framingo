package api

import "k8s.io/apimachinery/pkg/labels"

// SessionResponse is the wire view of the caller's own credential. Permissions
// are resolved per request rather than carried on entity.Credential: they are
// derived from Role and can change while a session is alive, so a copy stored
// at login would go stale.
type SessionResponse struct {
	Metadata             labels.Set `json:"metadata,omitempty"`
	Source               string     `json:"source"`
	Role                 string     `json:"role"`
	APIToken             string     `json:"api_token,omitempty"`
	AgentID              string     `json:"agent_id,omitempty"`
	UserID               int32      `json:"user_id,omitempty"`
	UserName             string     `json:"user_name,omitempty"`
	RequirePasswordReset bool       `json:"require_password_reset"`
	Permissions          []string   `json:"permissions"`
}

type LoginRequest struct {
	Username string `json:"username" form:"username" validate:"required"`
	Password string `json:"password" form:"password" validate:"required"`
}

type LoginResponse struct {
	RequirePasswordReset bool `json:"require_password_reset"`
}
