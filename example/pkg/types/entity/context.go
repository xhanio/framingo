package entity

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
)

// CredentialFrom returns the credential ctx acts for, and false when it
// carries none. The authn middleware puts it in echo's request store, which
// the project api.Context answers string-keyed lookups from: it is on the c a
// handler passes down and every ctx derived from it, not on
// c.Request().Context().
func CredentialFrom(ctx context.Context) (*Credential, bool) {
	cred, ok := ctx.Value(common.ContextKeyCredential).(*Credential)
	return cred, ok && cred != nil
}
