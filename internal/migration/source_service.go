package migration

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type sourceService struct {
	repo SourceRepo

	importCache *util.Cache[string, int]
}

var _ SourceService = &sourceService{}

func NewSourceService(repo SourceRepo) sourceService {
	return sourceService{
		repo:        repo,
		importCache: util.NewCache[string, int](),
	}
}

func (s sourceService) InitImportCache(initial map[string]int) error {
	return s.importCache.Replace(initial)
}

func (s sourceService) GetCachedImports(sourceName string) int {
	val, _ := s.importCache.Read(sourceName)
	return val
}

func (s sourceService) RecordActiveImport(sourceName string) {
	s.importCache.Write(sourceName, 1, func(existingVal, newVal int) int {
		return existingVal + newVal
	})
}

func (s sourceService) RemoveActiveImport(sourceName string) {
	s.importCache.Write(sourceName, 1, func(existingVal, newVal int) int {
		if existingVal > 0 {
			return newVal
		}

		return existingVal
	})
}

func (s sourceService) Create(ctx context.Context, newSource Source) (Source, error) {
	err := newSource.Validate()
	if err != nil {
		return Source{}, err
	}

	err = s.updateSourceConnectivity(ctx, &newSource)
	if err != nil {
		return Source{}, err
	}

	newSource.ID, err = s.repo.Create(ctx, newSource)
	if err != nil {
		return Source{}, err
	}

	return newSource, nil
}

func (s sourceService) GetAll(ctx context.Context, sourceTypes ...api.SourceType) (Sources, error) {
	return s.repo.GetAll(ctx, sourceTypes...)
}

func (s sourceService) GetAllNames(ctx context.Context, sourceTypes ...api.SourceType) ([]string, error) {
	return s.repo.GetAllNames(ctx, sourceTypes...)
}

func (s sourceService) GetByName(ctx context.Context, name string) (*Source, error) {
	if name == "" {
		return nil, fmt.Errorf("Source name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

func (s sourceService) Update(ctx context.Context, name string, newSource *Source, instanceService InstanceService) error {
	err := newSource.Validate()
	if err != nil {
		return err
	}

	// Reset connectivity status to trigger a scan on update.
	newSource.SetExternalConnectivityStatus(api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN)

	err = s.updateSourceConnectivity(ctx, newSource)
	if err != nil {
		return err
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		if instanceService != nil {
			_, err := s.canBeModified(ctx, name, instanceService)
			if err != nil {
				return fmt.Errorf("Unable to update source %q: %w", newSource.Name, err)
			}
		}

		return s.repo.Update(ctx, name, *newSource)
	})
}

func (s sourceService) DeleteByName(ctx context.Context, name string, instanceService InstanceService) error {
	if name == "" {
		return fmt.Errorf("Source name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		if instanceService != nil {
			instances, err := s.canBeModified(ctx, name, instanceService)
			if err != nil {
				return fmt.Errorf("Unable to remove source %q: %w", name, err)
			}

			for _, instanceUUID := range instances {
				err = instanceService.DeleteByUUID(ctx, instanceUUID)
				if err != nil {
					return fmt.Errorf("Unable to remove instance %q for source %q: %w", instanceUUID.String(), name, err)
				}
			}
		}

		return s.repo.DeleteByName(ctx, name)
	})
}

// canBeModified verifies whether the source with the given name can be modified, given its current instance states.
func (s sourceService) canBeModified(ctx context.Context, sourceName string, instanceService InstanceService) ([]uuid.UUID, error) {
	instances, err := instanceService.GetAllUUIDsBySource(ctx, sourceName)
	if err != nil {
		return nil, fmt.Errorf("Failed to get instances for source %q: %w", sourceName, err)
	}

	for _, instanceUUID := range instances {
		batches, err := instanceService.GetBatchesByUUID(ctx, instanceUUID)
		if err != nil {
			return nil, err
		}

		if len(batches) > 0 {
			return nil, fmt.Errorf("Instance %q cannot be modified because it is part of a batch", instanceUUID)
		}
	}

	return instances, nil
}

func (s sourceService) updateSourceConnectivity(ctx context.Context, src *Source) error {
	// Skip if source already has good connectivity.
	if src.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_OK {
		return nil
	}

	if src.EndpointFunc == nil {
		return fmt.Errorf("Endpoint function not defined for Source %q", src.Name)
	}

	endpoint, err := src.EndpointFunc(src.ToAPI())
	if err != nil {
		return err
	}

	// Do a basic connectivity check.
	status, untrustedCert := endpoint.DoBasicConnectivityCheck()

	if untrustedCert != nil && src.GetServerCertificate() == nil {
		// We got an untrusted certificate; if one hasn't already been set, add it to this source.
		src.SetServerCertificate(untrustedCert)
	}

	if status == api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT {
		// Need to wait for user to confirm if the fingerprint is trusted or not.
		src.SetExternalConnectivityStatus(status)
	} else if status != api.EXTERNALCONNECTIVITYSTATUS_OK {
		// Some other basic connectivity issue occurred.
		src.SetExternalConnectivityStatus(status)
	} else {
		// Basic connectivity is good, now test authentication.

		// Test the connectivity of this source.
		src.SetExternalConnectivityStatus(api.MapExternalConnectivityStatusToStatus(endpoint.Connect(ctx)))
	}

	return nil
}
