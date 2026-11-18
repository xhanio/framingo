package entity

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
)

// CredentialFrom reads echo's request store via api.Context: pass the handler's c, not c.Request().Context().
func CredentialFrom(ctx context.Context) (*Credential, bool) {
	cred, ok := ctx.Value(common.ContextKeyCredential).(*Credential)
	return cred, ok && cred != nil
}
