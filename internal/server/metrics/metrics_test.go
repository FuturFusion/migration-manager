package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/server/metrics"
)

func TestMetricSetString(t *testing.T) {
	metricSet := metrics.NewMetricSet()
	metricSet.Add(metrics.Batches, 2, metrics.Labels{"status": "Running"})
	metricSet.Add(metrics.Batches, 1, metrics.Labels{"status": `weird "name"`})
	metricSet.Add(metrics.GoGoroutines, 12, nil)
	metricSet.AddRaw([]byte("node_boot_time_seconds 1.7e+09"))

	expected := `# HELP migration_manager_batches The number of defined batches.
# TYPE migration_manager_batches gauge
migration_manager_batches{status="Running"} 2
migration_manager_batches{status="weird \"name\""} 1
# HELP migration_manager_go_goroutines The number of goroutines that currently exist.
# TYPE migration_manager_go_goroutines gauge
migration_manager_go_goroutines 12
node_boot_time_seconds 1.7e+09
# EOF
`

	require.Equal(t, expected, metricSet.String())
}

func TestAddCounts(t *testing.T) {
	metricSet := metrics.NewMetricSet()
	metrics.AddCounts(metricSet, metrics.Targets, []string{"incus", "incus", "other"}, func(targetType string) metrics.Labels {
		return metrics.Labels{"type": targetType}
	})

	expected := `# HELP migration_manager_targets The number of configured targets.
# TYPE migration_manager_targets gauge
migration_manager_targets{type="incus"} 2
migration_manager_targets{type="other"} 1
# EOF
`

	require.Equal(t, expected, metricSet.String())
}
