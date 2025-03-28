package migration

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type sourceService struct {
	repo SourceRepo
}

var _ SourceService = &sourceService{}

func NewSourceService(repo SourceRepo) sourceService {
	return sourceService{
		repo: repo,
	}
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

func (s sourceService) GetAll(ctx context.Context) (Sources, error) {
	return s.repo.GetAll(ctx)
}

func (s sourceService) GetAllNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllNames(ctx)
}

func (s sourceService) GetByName(ctx context.Context, name string) (*Source, error) {
	if name == "" {
		return nil, fmt.Errorf("Source name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

func (s sourceService) Update(ctx context.Context, newSource *Source, instanceService InstanceService) error {
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
			err := s.canBeModified(ctx, newSource.Name, instanceService)
			if err != nil {
				return fmt.Errorf("Unable to update source %q: %w", newSource.Name, err)
			}
		}

		return s.repo.Update(ctx, *newSource)
	})
}

func (s sourceService) DeleteByName(ctx context.Context, name string, instanceService InstanceService) error {
	if name == "" {
		return fmt.Errorf("Source name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		if instanceService != nil {
			err := s.canBeModified(ctx, name, instanceService)
			if err != nil {
				return fmt.Errorf("Unable to remove source %q: %w", name, err)
			}
		}

		return s.repo.DeleteByName(ctx, name)
	})
}

// canBeModified verifies whether the source with the given name can be modified, given its current instance states.
func (s sourceService) canBeModified(ctx context.Context, sourceName string, instanceService InstanceService) error {
	instances, err := instanceService.GetAllBySource(ctx, sourceName, false)
	if err != nil {
		return fmt.Errorf("Failed to get instances for source %q: %w", sourceName, err)
	}

	for _, instance := range instances {
		if !instance.CanBeModified() {
			return fmt.Errorf("Some instances cannot be modified (Status: %q)", instance.MigrationStatus.String())
		}
	}

	return nil
}

func (s sourceService) updateSourceConnectivity(ctx context.Context, src *Source) error {
	// Skip if source already has good connectivity.
	if src.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_OK {
		return nil
	}

	if src.EndpointFunc == nil {
		return fmt.Errorf("Endpoint function not defined for Source %q", src.Name)
	}

	endpoint, err := src.EndpointFunc(api.Source{
		Name:       src.Name,
		SourceType: src.SourceType,
		Properties: src.Properties,
	})
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
