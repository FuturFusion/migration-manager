package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (r vropsResource) instanceUUID() uuid.UUID {
	if r.ResourceKey.AdapterKind != "VMWARE" || r.ResourceKey.ResourceKind != "VirtualMachine" {
		return uuid.Nil
	}

	for _, identifier := range r.ResourceKey.Identifiers {
		if identifier.Type.Name == "VMEntityInstanceUUID" {
			id, err := uuid.Parse(identifier.Value)
			if err == nil {
				return id
			}
		}
	}

	return uuid.Nil
}

// CollectMonitoring matches VM instance UUIDs and retains successful results on partial failure.
func (s *InternalVROpsSource) CollectMonitoring(ctx context.Context, instances migration.Instances) (map[uuid.UUID]api.InstanceMonitoring, error) {
	ctx, cancel := context.WithTimeout(ctx, s.SyncTimeout.Duration)
	defer cancel()

	requested := map[uuid.UUID]bool{}
	for _, instance := range instances {
		if instance.SourceType == api.SOURCETYPE_VMWARE {
			requested[instance.UUID] = true
		}
	}

	result := map[uuid.UUID]api.InstanceMonitoring{}
	if len(requested) == 0 {
		return result, nil
	}

	byUUID := map[uuid.UUID][]vropsResource{}
	seen := map[string]bool{}
	for page := 0; ; page++ {
		response, err := s.getResources(ctx, page, 1000)
		if err != nil {
			return result, err
		}

		for _, resource := range response.ResourceList {
			if resource.Identifier == "" {
				continue
			}

			if seen[resource.Identifier] {
				return result, fmt.Errorf("vROps returned duplicate resource %q during pagination", resource.Identifier)
			}

			seen[resource.Identifier] = true
			id := resource.instanceUUID()
			if id != uuid.Nil && requested[id] {
				byUUID[id] = append(byUUID[id], resource)
			}
		}

		if len(response.ResourceList) == 0 {
			break
		}

		if response.PageInfo != nil && response.PageInfo.TotalCount != nil {
			if len(seen) >= *response.PageInfo.TotalCount {
				break
			}

			continue
		}

		pageSize := 1000
		if response.PageInfo != nil && response.PageInfo.PageSize > 0 {
			pageSize = response.PageInfo.PageSize
		}

		if len(response.ResourceList) < pageSize {
			break
		}
	}

	var failures []error
	for id, resources := range byUUID {
		if len(resources) != 1 {
			failures = append(failures, fmt.Errorf("multiple vROps resources match VM %s; skipping", id))
			continue
		}

		resource := resources[0]
		data := api.InstanceMonitoring{
			Source: s.Name, ResourceID: resource.Identifier,
			Health: resource.Health, AttemptedAt: time.Now().UTC(),
		}

		if len(resource.StatusStates) > 0 {
			data.Status = resource.StatusStates[0].Status
		}

		metrics, sampleTimes, err := s.getLatestStats(ctx, resource.Identifier)
		if err != nil {
			data.Error = "Unable to refresh monitoring data; see source sync warnings."
			failures = append(failures, fmt.Errorf("VM %s: %w", id, err))
		} else {
			data.Metrics = metrics
			data.SampleTimes = sampleTimes
			data.CollectedAt = time.Now().UTC()
		}

		result[id] = data
		if ctx.Err() != nil {
			return result, errors.Join(append(failures, ctx.Err())...)
		}
	}

	return result, errors.Join(failures...)
}
