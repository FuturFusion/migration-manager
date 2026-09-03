package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"runtime"
	"slices"
	"time"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/metrics"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// incusOSMetricsURL is the node exporter shipped with Incus OS.
const incusOSMetricsURL = "http://127.0.0.1:9100/metrics"

// incusOSMetricsSizeLimit is the maximum amount of Incus OS metrics data we accept.
const incusOSMetricsSizeLimit = 8 * 1024 * 1024

// startTime is set when the daemon binary starts, and is reported as its uptime.
var startTime = time.Now()

var metricsCmd = APIEndpoint{
	Path: "metrics",

	Get: APIEndpointAction{Handler: metricsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

// swagger:operation GET /1.0/metrics metrics metrics_get
//
//	Get metrics
//
//	Gets the daemon metrics in the OpenMetrics text format.
//
//	---
//	produces:
//	  - text/plain
//	responses:
//	  "200":
//	    description: Metrics
//	    schema:
//	      type: string
//	      description: Daemon metrics
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func metricsGet(d *Daemon, r *http.Request) response.Response {
	ctx := r.Context()

	metricSet := metrics.NewMetricSet()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		snapshot, err := loadStatsSnapshot(ctx, d)
		if err != nil {
			return err
		}

		addStatsMetrics(metricSet, snapshot)

		return d.addEntityMetrics(ctx, metricSet)
	})
	if err != nil {
		return response.SmartError(err)
	}

	addRuntimeMetrics(metricSet, time.Since(startTime))

	// On Incus OS, include the metrics of the OS itself.
	if util.IsIncusOS() {
		osMetrics, err := getIncusOSMetrics(ctx)
		if err != nil {
			slog.Warn("Failed to get Incus OS metrics", slog.Any("error", err))
		} else {
			metricSet.AddRaw(osMetrics)
		}
	}

	return response.ManualResponse(func(w http.ResponseWriter) error {
		w.Header().Set("Content-Type", "text/plain")
		_, err := io.WriteString(w, metricSet.String())
		return err
	})
}

// addStatsMetrics adds the metrics derived from the same overview as the stats endpoint.
func addStatsMetrics(metricSet *metrics.MetricSet, snapshot *statsSnapshot) {
	sourceTypes := make(map[string]string, len(snapshot.sources))
	for _, source := range snapshot.sources {
		sourceTypes[source.Name] = string(source.SourceType)
	}

	metrics.AddCounts(metricSet, metrics.Sources, snapshot.sources, func(source migration.Source) metrics.Labels {
		return metrics.Labels{"type": string(source.SourceType)}
	})

	metrics.AddCounts(metricSet, metrics.Batches, snapshot.batches, func(batch migration.Batch) metrics.Labels {
		return metrics.Labels{"status": string(batch.Status)}
	})

	queueEntries := slices.Collect(maps.Values(snapshot.queueEntryByInstance))
	metrics.AddCounts(metricSet, metrics.QueueEntries, queueEntries, func(queueEntry migration.QueueEntry) metrics.Labels {
		return metrics.Labels{"batch": queueEntry.BatchName, "status": string(queueEntry.MigrationStatus)}
	})

	sources := snapshot.sourcesOverview()

	metricSet.Add(metrics.Instances, float64(sources.TotalInstances), nil)

	for _, source := range sources.Items {
		labels := metrics.Labels{"source": source.Name}

		metricSet.Add(metrics.SourceInstances, float64(source.TotalInstances), metrics.Labels{"source": source.Name, "type": sourceTypes[source.Name]})
		metricSet.Add(metrics.SourceImportedInstances, float64(source.ImportedInstances), labels)
		metricSet.Add(metrics.SourceMigratedInstances, float64(source.MigratedInstances), labels)

		for _, ineligible := range source.IneligibleReasons {
			metricSet.Add(metrics.SourceIneligibleInstances, float64(len(ineligible.Instances)), metrics.Labels{"source": source.Name, "reason": ineligible.Reason})
		}

		importIssues := map[string][]api.InstanceRef{
			"blocked": source.BlockedImports,
			"failed":  source.FailedImports,
			"partial": source.PartialImports,
		}

		for issue, instances := range importIssues {
			metricSet.Add(metrics.SourceImportIssues, float64(len(instances)), metrics.Labels{"source": source.Name, "issue": issue})
		}
	}

	for _, batch := range snapshot.batchesOverview().Items {
		labels := metrics.Labels{"batch": batch.Name, "status": string(batch.Status)}

		metricSet.Add(metrics.BatchInstances, float64(batch.TotalInstances), labels)
		metricSet.Add(metrics.BatchMigratedInstances, float64(batch.MigratedInstances), labels)
		metricSet.Add(metrics.BatchDiskBytes, float64(batch.TotalDiskSize), labels)
		metricSet.Add(metrics.BatchMigratedDiskBytes, float64(batch.MigratedDiskSize), labels)

		for _, blocked := range batch.BlockedReasons {
			blockedLabels := maps.Clone(labels)
			blockedLabels["reason"] = blocked.Reason

			metricSet.Add(metrics.BatchBlockedInstances, float64(len(blocked.Instances)), blockedLabels)
		}
	}
}

// addEntityMetrics adds the metrics for the entities that the stats overview doesn't cover.
func (d *Daemon) addEntityMetrics(ctx context.Context, metricSet *metrics.MetricSet) error {
	networks, err := d.network.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get networks: %w", err)
	}

	metrics.AddCounts(metricSet, metrics.Networks, networks, func(network migration.Network) metrics.Labels {
		return metrics.Labels{"source": network.Source}
	})

	targets, err := d.target.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get targets: %w", err)
	}

	metrics.AddCounts(metricSet, metrics.Targets, targets, func(target migration.Target) metrics.Labels {
		return metrics.Labels{"type": string(target.TargetType)}
	})

	warnings, err := d.warning.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get warnings: %w", err)
	}

	metrics.AddCounts(metricSet, metrics.Warnings, warnings, func(warning migration.Warning) metrics.Labels {
		return metrics.Labels{"type": string(warning.Type), "status": string(warning.Status)}
	})

	artifacts, err := d.artifact.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get artifacts: %w", err)
	}

	metrics.AddCounts(metricSet, metrics.Artifacts, artifacts, func(artifact migration.Artifact) metrics.Labels {
		return metrics.Labels{"type": string(artifact.Type)}
	})

	return nil
}

// addRuntimeMetrics adds the metrics describing the daemon process itself.
func addRuntimeMetrics(metricSet *metrics.MetricSet, uptime time.Duration) {
	var memStats runtime.MemStats

	runtime.ReadMemStats(&memStats)

	metricSet.Add(metrics.Uptime, uptime.Seconds(), nil)
	metricSet.Add(metrics.GoGoroutines, float64(runtime.NumGoroutine()), nil)
	metricSet.Add(metrics.GoAllocBytes, float64(memStats.Alloc), nil)
	metricSet.Add(metrics.GoAllocBytesTotal, float64(memStats.TotalAlloc), nil)
	metricSet.Add(metrics.GoHeapObjects, float64(memStats.HeapObjects), nil)
	metricSet.Add(metrics.GoSysBytes, float64(memStats.Sys), nil)
}

// getIncusOSMetrics returns the raw metrics reported by the Incus OS node exporter.
func getIncusOSMetrics(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, incusOSMetricsURL, nil)
	if err != nil {
		return nil, err
	}

	client := http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, incusOSMetricsSizeLimit))
}
