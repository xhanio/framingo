package entity

import (
	"time"
)

type SystemTime struct {
	Time      time.Time `json:"time"`
	StartTime time.Time `json:"start_time"`
	UpTime    int64     `json:"up_time"` // in milliseconds
}

type SystemInfo struct {
	ProductName    string               `json:"product_name"`
	ProductModel   string               `json:"product_model"`
	ProductVersion string               `json:"product_version"`
	GitBranch      string               `json:"git_branch"`
	GitTag         string               `json:"git_tag"`
	BuildVersion   string               `json:"build_version"`
	BuildType      string               `json:"build_type"`
	BuildDate      string               `json:"build_date"`
	BuildTime      string               `json:"build_time"`
	Firmware       string               `json:"firmware"`
	SerialNumber   string               `json:"serial_number"`
	Hostname       string               `json:"hostname"`
	Timezone       string               `json:"timezone"`
	IsDST          bool                 `json:"is_dst"`
	InDST          bool                 `json:"in_dst"`
	Maintenance    *MaintenanceSchedule `json:"maintenance,omitempty"`
}

type MaintenanceSchedule struct {
	BeginAt time.Time `json:"begin_at"`
	EndAt   time.Time `json:"end_at"`
}
