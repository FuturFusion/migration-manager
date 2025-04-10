package migration

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type targetService struct {
	repo TargetRepo
}

var _ TargetService = &targetService{}

func NewTargetService(repo TargetRepo) targetService {
	return targetService{
		repo: repo,
	}
}

func (s targetService) Create(ctx context.Context, newTarget Target) (Target, error) {
	err := newTarget.Validate()
	if err != nil {
		return Target{}, err
	}

	err = s.updateTargetConnectivity(ctx, &newTarget)
	if err != nil {
		return Target{}, err
	}

	newTarget.ID, err = s.repo.Create(ctx, newTarget)
	if err != nil {
		return Target{}, err
	}

	return newTarget, nil
}

func (s targetService) GetAll(ctx context.Context) (Targets, error) {
	return s.repo.GetAll(ctx)
}

func (s targetService) GetAllNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllNames(ctx)
}

func (s targetService) GetByName(ctx context.Context, name string) (*Target, error) {
	if name == "" {
		return nil, fmt.Errorf("Target name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

func (s targetService) Update(ctx context.Context, name string, newTarget *Target) error {
	err := newTarget.Validate()
	if err != nil {
		return err
	}

	// Reset connectivity status to trigger a scan on update.
	newTarget.SetExternalConnectivityStatus(api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN)

	err = s.updateTargetConnectivity(ctx, newTarget)
	if err != nil {
		return err
	}

	return s.repo.Update(ctx, name, *newTarget)
}

func (s targetService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("Target name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.DeleteByName(ctx, name)
}

func (s targetService) updateTargetConnectivity(ctx context.Context, tgt *Target) error {
	// Skip if target already has good connectivity.
	if tgt.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_OK {
		return nil
	}

	if tgt.EndpointFunc == nil {
		return fmt.Errorf("Endpoint function not defined for Target %q", tgt.Name)
	}

	endpoint, err := tgt.EndpointFunc(api.Target{
		Name:       tgt.Name,
		TargetType: tgt.TargetType,
		Properties: tgt.Properties,
	})
	if err != nil {
		return err
	}

	// Do a basic connectivity check.
	status, untrustedCert := endpoint.DoBasicConnectivityCheck()

	if untrustedCert != nil && tgt.GetServerCertificate() == nil {
		// We got an untrusted certificate; if one hasn't already been set, add it to this target.
		tgt.SetServerCertificate(untrustedCert)
	}

	if status == api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT {
		// Need to wait for user to confirm if the fingerprint is trusted or not.
		tgt.SetExternalConnectivityStatus(status)
	} else if status != api.EXTERNALCONNECTIVITYSTATUS_OK {
		// Some other basic connectivity issue occurred.
		tgt.SetExternalConnectivityStatus(status)
	} else {
		// Basic connectivity is good, now test authentication.

		if endpoint.IsWaitingForOIDCTokens() {
			// Target is configured for OIDC, but has no tokens yet.
			tgt.SetExternalConnectivityStatus(api.EXTERNALCONNECTIVITYSTATUS_WAITING_OIDC)
		} else {
			// Test the connectivity of this target.
			tgt.SetExternalConnectivityStatus(api.MapExternalConnectivityStatusToStatus(endpoint.Connect(ctx)))
		}
	}

	return nil
}
