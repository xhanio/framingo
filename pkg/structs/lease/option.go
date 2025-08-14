package lease

import "github.com/xhanio/framingo/pkg/utils/log"

type LeaseOption func(*lease)

func Once() LeaseOption {
	return func(l *lease) {
		l.once = true
	}
}

func UseWallTime() LeaseOption {
	return func(l *lease) {
		l.wall = true
	}
}

func WithLogger(logger log.Logger) LeaseOption {
	return func(l *lease) {
		l.log = logger
	}
}

func OnExpired(fn func()) LeaseOption {
	return func(l *lease) {
		l.onExpire = append(l.onExpire, fn)
	}
}

func OnCancel(fn func()) LeaseOption {
	return func(l *lease) {
		l.onCancel = append(l.onCancel, fn)
	}
}

func OnRefresh(fn func()) LeaseOption {
	return func(l *lease) {
		l.onRefresh = append(l.onRefresh, fn)
	}
}

func OnExtend(fn func()) LeaseOption {
	return func(l *lease) {
		l.onExtend = append(l.onExtend, fn)
	}
}

func OnRenew(fn func()) LeaseOption {
	return func(l *lease) {
		l.onRenew = append(l.onRenew, fn)
	}
}
