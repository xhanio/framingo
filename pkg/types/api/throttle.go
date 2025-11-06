package api

import "golang.org/x/time/rate"

type ThrottleConfig struct {
	RPS       rate.Limit `json:"rps"`
	BurstSize int        `json:"burst_size"`
}
