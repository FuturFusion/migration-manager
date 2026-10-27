package source

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/FuturFusion/migration-manager/shared/api"
)

var vropsSummaryStatKeys = []string{
	"cpu|usage_average", "mem|usage_average",
	"guestfilesystem|usage_total", "guestfilesystem|capacity_total",
	"storage|usage_average", "storage|commandsAveraged_average", "net|transmitted_average",
}

// ParseVROpsMonitoringPeriod accepts only the supported summary periods.
func ParseVROpsMonitoringPeriod(period string) (time.Duration, error) {
	switch period {
	case "", "10m":
		return 10 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	case "1w":
		return 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("Invalid monitoring period %q; use 10m, 1h, 1d or 1w", period)
	}
}

// GetMonitoringSummary queries vROps without updating the stored monitoring snapshot.
func (s *InternalVROpsSource) GetMonitoringSummary(ctx context.Context, resourceID string, period string) (api.InstanceMonitoringSummary, error) {
	duration, err := ParseVROpsMonitoringPeriod(period)
	if err != nil {
		return api.InstanceMonitoringSummary{}, err
	}

	end := time.Now().UTC()
	history, err := s.getMonitoringHistory(ctx, resourceID, vropsSummaryStatKeys, end.Add(-duration), end)
	if err != nil {
		return api.InstanceMonitoringSummary{}, err
	}

	summary := api.InstanceMonitoringSummary{Begin: history.Begin, End: history.End, Metrics: map[string]float64{}}
	for key, samples := range history.Metrics {
		samples = slices.CompactFunc(samples, func(a api.InstanceMonitoringSample, b api.InstanceMonitoringSample) bool {
			return a.Timestamp.Equal(b.Timestamp)
		})
		if len(samples) == 0 {
			continue
		}

		if key == "guestfilesystem|usage_total" || key == "guestfilesystem|capacity_total" {
			summary.Metrics[key] = samples[len(samples)-1].Value
			continue
		}

		mean := 0.0
		for i, sample := range samples {
			mean += (sample.Value - mean) / float64(i+1)
		}

		summary.Metrics[key] = mean
	}

	return summary, nil
}
