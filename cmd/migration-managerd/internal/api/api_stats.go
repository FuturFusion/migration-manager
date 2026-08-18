package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var statsCmd = APIEndpoint{
	Path: "stats",

	Get: APIEndpointAction{Handler: statsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

// swagger:operation GET /1.0/stats stats stats_get
//
//	Get a high-level overview of migration manager
//
//	Returns a summary of batches, instances, and disk usage.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Migration manager stats overview
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/StatsOverview"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func statsGet(d *Daemon, r *http.Request) response.Response {
	var result api.StatsOverview
	err := transaction.Do(r.Context(), func(ctx context.Context) error {
		snapshot, err := loadStatsSnapshot(ctx, d)
		if err != nil {
			return err
		}

		result = api.StatsOverview{
			Sources: snapshot.sourcesOverview(),
			Batches: snapshot.batchesOverview(),
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, result)
}

// statsSnapshot is a consistent point-in-time view of the state the stats are derived from.
type statsSnapshot struct {
	sources   migration.Sources
	batches   migration.Batches
	instances migration.Instances

	instancesByBatch     map[string]migration.Instances
	instanceByUUID       map[uuid.UUID]migration.Instance
	batchByName          map[string]migration.Batch
	queueEntryByInstance map[uuid.UUID]migration.QueueEntry
	windowsByBatch       map[string]migration.Windows
	syncWarningsBySource map[string]map[api.WarningType]bool
}

// loadStatsSnapshot reads everything the stats are derived from, and indexes it for lookup.
func loadStatsSnapshot(ctx context.Context, d *Daemon) (*statsSnapshot, error) {
	snapshot := &statsSnapshot{}

	var err error
	snapshot.sources, err = d.source.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	snapshot.batches, err = d.batch.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	snapshot.instances, err = d.instance.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	snapshot.instanceByUUID = make(map[uuid.UUID]migration.Instance, len(snapshot.instances))
	for _, instance := range snapshot.instances {
		snapshot.instanceByUUID[instance.UUID] = instance
	}

	snapshot.batchByName = make(map[string]migration.Batch, len(snapshot.batches))
	for _, batch := range snapshot.batches {
		snapshot.batchByName[batch.Name] = batch
	}

	// Instances may be assigned to a batch before a queue entry exists, so they're looked up per batch.
	snapshot.instancesByBatch = make(map[string]migration.Instances, len(snapshot.batches))
	for _, batch := range snapshot.batches {
		snapshot.instancesByBatch[batch.Name], err = d.instance.GetAllByBatch(ctx, batch.Name)
		if err != nil {
			return nil, err
		}
	}

	queueEntries, err := d.queue.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	snapshot.queueEntryByInstance = make(map[uuid.UUID]migration.QueueEntry, len(queueEntries))
	for _, queueEntry := range queueEntries {
		snapshot.queueEntryByInstance[queueEntry.InstanceUUID] = queueEntry
	}

	windows, err := d.window.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	snapshot.windowsByBatch = make(map[string]migration.Windows, len(windows))
	for _, window := range windows {
		snapshot.windowsByBatch[window.Batch] = append(snapshot.windowsByBatch[window.Batch], window)
	}

	warnings, err := d.warning.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// A sync warning is deleted once a clean sync stops reporting it, so any that remain describe the latest sync.
	syncScope := api.WarningScopeSync()
	snapshot.syncWarningsBySource = make(map[string]map[api.WarningType]bool)
	for _, warning := range warnings {
		if !syncScope.Match(warning.ToAPI()) {
			continue
		}

		if snapshot.syncWarningsBySource[warning.Entity] == nil {
			snapshot.syncWarningsBySource[warning.Entity] = map[api.WarningType]bool{}
		}

		snapshot.syncWarningsBySource[warning.Entity][warning.Type] = true
	}

	return snapshot, nil
}

// batchesOverview computes the system-wide batch totals and the per-batch breakdown.
func (s *statsSnapshot) batchesOverview() api.BatchesOverview {
	overview := api.BatchesOverview{
		Items: make([]api.BatchSummary, 0, len(s.batches)),
	}

	overviewBlockedReasonInstances := make(map[string][]api.InstanceRef)

	// An instance may be assigned to several batches, so only count it once towards the totals.
	counted := make(map[uuid.UUID]bool)

	for _, batch := range s.batches {
		batchInstances := s.instancesByBatch[batch.Name]

		stats := api.BatchSummary{
			Name:           batch.Name,
			Status:         batch.Status,
			TotalInstances: len(batchInstances),
		}

		blockedReasonInstances := make(map[string][]api.InstanceRef)

		for _, instance := range batchInstances {
			var diskSize int64
			for _, disk := range instance.Properties.Disks {
				diskSize += disk.Capacity
			}

			stats.TotalDiskSize += diskSize

			queueEntry, queued := s.queueEntryByInstance[instance.UUID]

			// Once background import completes the disk is on the target, even if the migration hasn't finished.
			transferred := queued && queueEntry.ImportStage != migration.IMPORTSTAGE_BACKGROUND
			migrated := queued && queueEntry.MigrationStatus == api.MIGRATIONSTATUS_FINISHED
			blocked := queued && queueEntry.MigrationStatus == api.MIGRATIONSTATUS_BLOCKED

			// An instance has a single queue entry, owned by whichever batch took control of it first.
			if queued && queueEntry.BatchName == batch.Name {
				if transferred {
					stats.MigratedDiskSize += diskSize
				}

				if migrated {
					stats.MigratedInstances++
				}

				if blocked {
					stats.BlockedInstances++
					blockedReason := s.blockedReason(queueEntry)
					blockedReasonInstances[blockedReason] = append(blockedReasonInstances[blockedReason], instanceRef(instance))
				}
			}

			if !counted[instance.UUID] {
				counted[instance.UUID] = true

				overview.TotalInstances++
				overview.TotalDiskSize += diskSize

				if transferred {
					overview.MigratedDiskSize += diskSize
				}

				if migrated {
					overview.MigratedInstances++
				}

				if blocked {
					overview.BlockedInstances++
					blockedReason := s.blockedReason(queueEntry)
					overviewBlockedReasonInstances[blockedReason] = append(overviewBlockedReasonInstances[blockedReason], instanceRef(instance))
				}
			}
		}

		// Migration windows only take effect once a batch is actually running.
		if batch.Status == api.BATCHSTATUS_RUNNING {
			stats.NextWindow = nextBatchWindow(s.windowsByBatch[batch.Name])
		}

		stats.BlockedReasons = reasonInstances(blockedReasonInstances)
		overview.Items = append(overview.Items, stats)
	}

	overview.BlockedReasons = reasonInstances(overviewBlockedReasonInstances)

	sort.Slice(overview.Items, func(i, j int) bool {
		return overview.Items[i].Name < overview.Items[j].Name
	})

	return overview
}

// overridingBatch returns the name of the first batch whose overrides waive the instance's restriction.
func overridingBatch(instance migration.Instance, batches []migration.Batch) string {
	for _, batch := range batches {
		if instance.DisabledReason(batch.Config.RestrictionOverrides) == nil {
			return batch.Name
		}
	}

	return ""
}

// blockedReason categorises a blocked queue entry, preferring the recorded message over the restriction check.
func (s *statsSnapshot) blockedReason(queueEntry migration.QueueEntry) string {
	for _, prefix := range []string{blockedReasonArtifact, blockedReasonFilesystem, blockedReasonPlacement} {
		if strings.HasPrefix(queueEntry.MigrationStatusMessage, prefix+":") {
			return prefix
		}
	}

	instance, ok := s.instanceByUUID[queueEntry.InstanceUUID]
	if ok {
		var overrides api.InstanceRestrictionOverride
		batch, ok := s.batchByName[queueEntry.BatchName]
		if ok {
			overrides = batch.Config.RestrictionOverrides
		}

		var disabledErr migration.ErrDisabled
		if errors.As(instance.DisabledReason(overrides), &disabledErr) {
			return string(disabledErr.Reason())
		}
	}

	return blockedReasonUnknown
}

// nextBatchWindow returns the batch's soonest migration window that hasn't ended, nil if it has none.
func nextBatchWindow(windows migration.Windows) *api.BatchWindow {
	var next *migration.Window
	for _, window := range windows {
		if window.Ended() {
			continue
		}

		if next == nil || window.Start.Before(next.Start) {
			next = &window
		}
	}

	if next == nil {
		return nil
	}

	return &api.BatchWindow{
		Name:  next.Name,
		Start: next.Start,
		End:   next.End,
	}
}

// sourcesOverview computes the system-wide source totals and the per-source breakdown.
func (s *statsSnapshot) sourcesOverview() api.SourcesOverview {
	overview := api.SourcesOverview{
		TotalInstances:      len(s.instances),
		OverriddenInstances: []api.OverriddenInstance{},
		BlockedImports:      []api.InstanceRef{},
		FailedImports:       []api.InstanceRef{},
		PartialImports:      []api.InstanceRef{},
	}

	summaryBySource := make(map[string]*api.SourceSummary, len(s.sources))
	ineligibleReasonInstancesBySource := make(map[string]map[string][]api.InstanceRef, len(s.sources))
	for _, source := range s.sources {
		summaryBySource[source.Name] = &api.SourceSummary{
			Name:                source.Name,
			ConnectivityStatus:  source.GetExternalConnectivityStatus(),
			OverriddenInstances: []api.OverriddenInstance{},
			BlockedImports:      []api.InstanceRef{},
			FailedImports:       []api.InstanceRef{},
			PartialImports:      []api.InstanceRef{},
		}

		ineligibleReasonInstancesBySource[source.Name] = make(map[string][]api.InstanceRef)
	}

	overviewIneligibleReasonInstances := make(map[string][]api.InstanceRef)
	notImportedBySource := make(map[string]int, len(s.sources))
	notImported := 0

	// Overrides are reported as a subset of the ineligible count, so the count itself stays batch agnostic.
	batchesByInstance := make(map[uuid.UUID][]migration.Batch)
	for _, batch := range s.batches {
		for _, instance := range s.instancesByBatch[batch.Name] {
			batchesByInstance[instance.UUID] = append(batchesByInstance[instance.UUID], batch)
		}
	}

	for _, instance := range s.instances {
		// Nil when the instance's source has since been removed, leaving it counted only in the totals.
		summary := summaryBySource[instance.Source]
		if summary != nil {
			summary.TotalInstances++
		}

		queueEntry, queued := s.queueEntryByInstance[instance.UUID]
		migrated := queued && queueEntry.MigrationStatus == api.MIGRATIONSTATUS_FINISHED

		// Eligibility is a property of the instance itself, so batch overrides don't apply; migrating is what clears it.
		if !migrated {
			var disabledErr migration.ErrDisabled
			if errors.As(instance.DisabledReason(api.InstanceRestrictionOverride{}), &disabledErr) {
				reason := string(disabledErr.Reason())

				overview.IneligibleInstances++
				overviewIneligibleReasonInstances[reason] = append(overviewIneligibleReasonInstances[reason], instanceRef(instance))

				if summary != nil {
					summary.IneligibleInstances++
					ineligibleReasonInstancesBySource[instance.Source][reason] = append(ineligibleReasonInstancesBySource[instance.Source][reason], instanceRef(instance))
				}

				// A full sync counts as a successful import, so only the remainder can be blocked by a warning.
				if unreadableProperty(disabledErr.Reason()) {
					reported := s.syncWarningsBySource[instance.Source]
					ref := instanceRef(instance)

					if reported[api.SourceUnavailable] {
						overview.BlockedImports = append(overview.BlockedImports, ref)

						if summary != nil {
							summary.BlockedImports = append(summary.BlockedImports, ref)
						}
					}

					if reported[api.InstanceImportFailed] {
						overview.FailedImports = append(overview.FailedImports, ref)

						if summary != nil {
							summary.FailedImports = append(summary.FailedImports, ref)
						}
					}

					if reported[api.InstanceIncomplete] {
						overview.PartialImports = append(overview.PartialImports, ref)

						if summary != nil {
							summary.PartialImports = append(summary.PartialImports, ref)
						}
					}

					// The warnings overlap, so an instance only drops out of the imported count once.
					if reported[api.SourceUnavailable] || reported[api.InstanceImportFailed] || reported[api.InstanceIncomplete] {
						notImported++
						notImportedBySource[instance.Source]++
					}
				}

				batchName := overridingBatch(instance, batchesByInstance[instance.UUID])
				if batchName != "" {
					overridden := api.OverriddenInstance{InstanceRef: instanceRef(instance), Batch: batchName}

					overview.OverriddenInstances = append(overview.OverriddenInstances, overridden)

					if summary != nil {
						summary.OverriddenInstances = append(summary.OverriddenInstances, overridden)
					}
				}
			}
		}

		if !queued {
			continue
		}

		if queueEntry.MigrationStatus == api.MIGRATIONSTATUS_FINISHED {
			overview.MigratedInstances++

			if summary != nil {
				summary.MigratedInstances++
			}
		}
	}

	overview.IneligibleReasons = reasonInstances(overviewIneligibleReasonInstances)
	sortOverriddenInstances(overview.OverriddenInstances)
	sortInstanceRefs(overview.BlockedImports)
	sortInstanceRefs(overview.FailedImports)
	sortInstanceRefs(overview.PartialImports)
	overview.ImportedInstances = overview.TotalInstances - notImported

	overview.Items = make([]api.SourceSummary, 0, len(summaryBySource))
	for name, summary := range summaryBySource {
		summary.IneligibleReasons = reasonInstances(ineligibleReasonInstancesBySource[name])
		sortOverriddenInstances(summary.OverriddenInstances)
		sortInstanceRefs(summary.BlockedImports)
		sortInstanceRefs(summary.FailedImports)
		sortInstanceRefs(summary.PartialImports)
		summary.ImportedInstances = summary.TotalInstances - notImportedBySource[name]
		overview.Items = append(overview.Items, *summary)
	}

	sort.Slice(overview.Items, func(i, j int) bool {
		return overview.Items[i].Name < overview.Items[j].Name
	})

	return overview
}

// instanceRef identifies an instance for display and linking.
func instanceRef(instance migration.Instance) api.InstanceRef {
	return api.InstanceRef{UUID: instance.UUID, Location: instance.Properties.Location}
}

// sortInstanceRefs orders instance references by location so the response is stable.
func sortInstanceRefs(refs []api.InstanceRef) {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Location < refs[j].Location
	})
}

// unreadableProperty reports whether a disabled reason means the source couldn't report a property.
func unreadableProperty(reason migration.DisabledReason) bool {
	switch reason {
	case migration.DISABLEDREASON_UNKNOWN_OS,
		migration.DISABLEDREASON_UNKNOWN_ARCHITECTURE,
		migration.DISABLEDREASON_UNKNOWN_IP_ADDRESS:
		return true
	default:
		return false
	}
}

// sortOverriddenInstances orders overridden instances by location so the response is stable.
func sortOverriddenInstances(instances []api.OverriddenInstance) {
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Location < instances[j].Location
	})
}

// reasonInstances converts a reason-to-instances map into a slice sorted by instance count, then reason.
func reasonInstances(byReason map[string][]api.InstanceRef) []api.ReasonInstances {
	stats := make([]api.ReasonInstances, 0, len(byReason))
	for reason, instances := range byReason {
		sortInstanceRefs(instances)

		stats = append(stats, api.ReasonInstances{Reason: reason, Instances: instances})
	}

	sort.Slice(stats, func(i, j int) bool {
		if len(stats[i].Instances) != len(stats[j].Instances) {
			return len(stats[i].Instances) > len(stats[j].Instances)
		}

		return stats[i].Reason < stats[j].Reason
	})

	return stats
}
