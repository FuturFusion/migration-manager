package source

import (
	"context"
	"math"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/FuturFusion/migration-manager/shared/api"
)

var vropsHistoryStatKeys = []string{"cpu|usage_average", "mem|usage_average", "disk|read_average", "disk|write_average"}

// GetMonitoringHistory fetches the last thirty minutes at the available source resolution.
func (s *InternalVROpsSource) GetMonitoringHistory(ctx context.Context, resourceID string) (api.InstanceMonitoringHistory, error) {
	end := time.Now().UTC()
	return s.getMonitoringHistory(ctx, resourceID, vropsHistoryStatKeys, end.Add(-30*time.Minute), end)
}

func (s *InternalVROpsSource) getMonitoringHistory(ctx context.Context, resourceID string, keys []string, begin time.Time, end time.Time) (api.InstanceMonitoringHistory, error) {
	query := url.Values{
		"statKey": keys,
		"begin":   {strconv.FormatInt(begin.UnixMilli(), 10)}, "end": {strconv.FormatInt(end.UnixMilli(), 10)},
		"metrics": {"true"}, "currentOnly": {"false"},
	}

	var response vropsStatsResponse

	history := api.InstanceMonitoringHistory{Begin: begin, End: end, Metrics: map[string][]api.InstanceMonitoringSample{}}
	path := "/resources/" + url.PathEscape(resourceID) + "/stats?" + query.Encode()
	err := s.getJSON(ctx, path, &response)
	if err != nil {
		return history, err
	}

	for _, resource := range response.Values {
		if resource.ResourceID != resourceID {
			continue
		}

		for _, stat := range resource.statistics() {
			key := stat.Key.Key
			if !slices.Contains(keys, key) || len(stat.Data) != len(stat.Timestamps) {
				continue
			}

			for i, timestamp := range stat.Timestamps {
				value := stat.Data[i]
				if timestamp < begin.UnixMilli() || timestamp > end.UnixMilli() || value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
					continue
				}

				history.Metrics[key] = append(history.Metrics[key], api.InstanceMonitoringSample{Timestamp: time.UnixMilli(timestamp).UTC(), Value: *value})
			}
		}
	}

	for key, samples := range history.Metrics {
		sort.SliceStable(samples, func(i, j int) bool { return samples[i].Timestamp.Before(samples[j].Timestamp) })
		history.Metrics[key] = samples
	}

	return history, nil
}
