package api

import "golang.org/x/time/rate"

type ThrottleConfig struct {
	RPS       rate.Limit
	BurstSize int
}
