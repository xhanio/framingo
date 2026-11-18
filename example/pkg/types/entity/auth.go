package entity

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/structs/lease"

	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

type Credential struct {
	Metadata             labels.Set `json:"metadata,omitempty"`
	Source               string     `json:"source"`
	Role                 string     `json:"role"`
	APIToken             string     `json:"api_token,omitempty"`
	AgentID              string     `json:"agent_id,omitempty"`
	UserID               int32      `json:"user_id,omitempty"`
	UserName             string     `json:"user_name,omitempty"`    // filled by user service
	RequirePasswordReset bool       `json:"require_password_reset"` // filled by user service
	OrganizationID       int32      `json:"-"`
	OrganizationName     string     `json:"-"` // filled by user service
}

// Credential is immutable and shared by every request on a session; derived state such as permissions belongs in the response.

func (c *Credential) UID() string {
	if c == nil {
		return ""
	}
	var subject string
	if c.APIToken != "" {
		subject = c.APIToken
	} else if c.AgentID != "" {
		subject = c.AgentID
	} else {
		subject = fmt.Sprintf("%s/%s", c.OrganizationName, c.UserName)
	}
	return fmt.Sprintf("%s:%s", c.Source, subject)
}

// LocalUserID is false unless a local user: API tokens and agents carry none, and an LDAP user's ID is not ours.
func (c *Credential) LocalUserID() (int32, bool) {
	if c == nil || c.Source != preset.AuthSourceLocalUser {
		return 0, false
	}
	return c.UserID, true
}

func (c *Credential) IsAdmin() bool {
	if c == nil {
		return false
	}
	return c.Role == rbac.RoleAdmin
}

type Session struct {
	ID         string      `json:"id"`
	Credential *Credential `json:"-"`
	Lease      lease.Lease `json:"-"`
}

func (s *Session) Key() string {
	if s == nil {
		return ""
	}
	return s.ID
}

func (s *Session) UID() string {
	if s == nil || len(s.ID) < 8 {
		return ""
	}
	return fmt.Sprintf("%s-%s", s.Credential.UID(), s.ID[:8])
}

type AuthListOptions struct {
	Metadata labels.Selector
	Source   string
	Role     string
}
