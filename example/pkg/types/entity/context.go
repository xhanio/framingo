package entity

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
)

// CredentialFrom reads the credential authentication put in the request's context, or a
// handler set in echo's store: pass the handler's c, which answers from both, or a
// context derived from it; a middleware passes c.Request().Context().
func CredentialFrom(ctx context.Context) (*Credential, bool) {
	cred, ok := ctx.Value(common.ContextKeyCredential).(*Credential)
	return cred, ok && cred != nil
}

// WithCredential is ctx carrying cred, for authentication to put in the request's
// context (design/request-context.md).
func WithCredential(ctx context.Context, cred *Credential) context.Context {
	//nolint:staticcheck // SA1029: framingo's keys are strings, shared with echo's store
	return context.WithValue(ctx, common.ContextKeyCredential, cred)
}

// SessionFrom reads the session authentication put in the request's context.
func SessionFrom(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(common.ContextKeySession).(*Session)
	return session, ok && session != nil
}

// WithSession is ctx carrying session, for authentication to put in the request's
// context (design/request-context.md).
func WithSession(ctx context.Context, session *Session) context.Context {
	//nolint:staticcheck // SA1029: framingo's keys are strings, shared with echo's store
	return context.WithValue(ctx, common.ContextKeySession, session)
}
