package lease

import "time"

type Lease interface {
	ID() string
	Start()
	Refresh(duraton time.Duration) bool
	Extend(duraton time.Duration) bool
	Renew(expiresAt time.Time) bool
	Cancel()
	Expired() bool
	ExpiresAt() time.Time
	Hooks
}

type Hooks interface {
	OnRefresh(fn func())
	OnExtend(fn func())
	OnRenew(fn func())
	OnExpired(fn func())
	OnCancel(fn func())
}
