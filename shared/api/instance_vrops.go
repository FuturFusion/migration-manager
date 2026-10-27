package api

import "time"

// InstanceMonitoringSample is a timestamped monitoring value.
type InstanceMonitoringSample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// InstanceMonitoringHistory contains on-demand monitoring samples.
//
// swagger:model
type InstanceMonitoringHistory struct {
	Begin   time.Time                             `json:"begin"`
	End     time.Time                             `json:"end"`
	Metrics map[string][]InstanceMonitoringSample `json:"metrics"`
}

// InstanceMonitoringSummary contains period averages and the latest filesystem measurements.
//
// swagger:model
type InstanceMonitoringSummary struct {
	Begin   time.Time          `json:"begin"`
	End     time.Time          `json:"end"`
	Metrics map[string]float64 `json:"metrics"`
}
