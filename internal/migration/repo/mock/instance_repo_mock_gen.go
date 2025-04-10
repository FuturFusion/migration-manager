// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mock

import (
	"context"
	"sync"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/google/uuid"
)

// Ensure, that InstanceRepoMock does implement migration.InstanceRepo.
// If this is not the case, regenerate this file with moq.
var _ migration.InstanceRepo = &InstanceRepoMock{}

// InstanceRepoMock is a mock implementation of migration.InstanceRepo.
//
//	func TestSomethingThatUsesInstanceRepo(t *testing.T) {
//
//		// make and configure a mocked migration.InstanceRepo
//		mockedInstanceRepo := &InstanceRepoMock{
//			CreateFunc: func(ctx context.Context, instance migration.Instance) (int64, error) {
//				panic("mock out the Create method")
//			},
//			CreateOverridesFunc: func(ctx context.Context, overrides migration.InstanceOverride) (int64, error) {
//				panic("mock out the CreateOverrides method")
//			},
//			DeleteByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
//				panic("mock out the DeleteByUUID method")
//			},
//			DeleteOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
//				panic("mock out the DeleteOverridesByUUID method")
//			},
//			GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
//				panic("mock out the GetAll method")
//			},
//			GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
//				panic("mock out the GetAllByBatch method")
//			},
//			GetAllByBatchAndStateFunc: func(ctx context.Context, batch string, status api.MigrationStatusType) (migration.Instances, error) {
//				panic("mock out the GetAllByBatchAndState method")
//			},
//			GetAllBySourceFunc: func(ctx context.Context, source string) (migration.Instances, error) {
//				panic("mock out the GetAllBySource method")
//			},
//			GetAllByStateFunc: func(ctx context.Context, status ...api.MigrationStatusType) (migration.Instances, error) {
//				panic("mock out the GetAllByState method")
//			},
//			GetAllUUIDsFunc: func(ctx context.Context) ([]uuid.UUID, error) {
//				panic("mock out the GetAllUUIDs method")
//			},
//			GetAllUnassignedFunc: func(ctx context.Context) (migration.Instances, error) {
//				panic("mock out the GetAllUnassigned method")
//			},
//			GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
//				panic("mock out the GetByUUID method")
//			},
//			GetOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
//				panic("mock out the GetOverridesByUUID method")
//			},
//			UpdateFunc: func(ctx context.Context, instance migration.Instance) error {
//				panic("mock out the Update method")
//			},
//			UpdateOverridesFunc: func(ctx context.Context, overrides migration.InstanceOverride) error {
//				panic("mock out the UpdateOverrides method")
//			},
//		}
//
//		// use mockedInstanceRepo in code that requires migration.InstanceRepo
//		// and then make assertions.
//
//	}
type InstanceRepoMock struct {
	// CreateFunc mocks the Create method.
	CreateFunc func(ctx context.Context, instance migration.Instance) (int64, error)

	// CreateOverridesFunc mocks the CreateOverrides method.
	CreateOverridesFunc func(ctx context.Context, overrides migration.InstanceOverride) (int64, error)

	// DeleteByUUIDFunc mocks the DeleteByUUID method.
	DeleteByUUIDFunc func(ctx context.Context, id uuid.UUID) error

	// DeleteOverridesByUUIDFunc mocks the DeleteOverridesByUUID method.
	DeleteOverridesByUUIDFunc func(ctx context.Context, id uuid.UUID) error

	// GetAllFunc mocks the GetAll method.
	GetAllFunc func(ctx context.Context) (migration.Instances, error)

	// GetAllByBatchFunc mocks the GetAllByBatch method.
	GetAllByBatchFunc func(ctx context.Context, batch string) (migration.Instances, error)

	// GetAllByBatchAndStateFunc mocks the GetAllByBatchAndState method.
	GetAllByBatchAndStateFunc func(ctx context.Context, batch string, status api.MigrationStatusType) (migration.Instances, error)

	// GetAllBySourceFunc mocks the GetAllBySource method.
	GetAllBySourceFunc func(ctx context.Context, source string) (migration.Instances, error)

	// GetAllByStateFunc mocks the GetAllByState method.
	GetAllByStateFunc func(ctx context.Context, status ...api.MigrationStatusType) (migration.Instances, error)

	// GetAllUUIDsFunc mocks the GetAllUUIDs method.
	GetAllUUIDsFunc func(ctx context.Context) ([]uuid.UUID, error)

	// GetAllUnassignedFunc mocks the GetAllUnassigned method.
	GetAllUnassignedFunc func(ctx context.Context) (migration.Instances, error)

	// GetByUUIDFunc mocks the GetByUUID method.
	GetByUUIDFunc func(ctx context.Context, id uuid.UUID) (*migration.Instance, error)

	// GetOverridesByUUIDFunc mocks the GetOverridesByUUID method.
	GetOverridesByUUIDFunc func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error)

	// UpdateFunc mocks the Update method.
	UpdateFunc func(ctx context.Context, instance migration.Instance) error

	// UpdateOverridesFunc mocks the UpdateOverrides method.
	UpdateOverridesFunc func(ctx context.Context, overrides migration.InstanceOverride) error

	// calls tracks calls to the methods.
	calls struct {
		// Create holds details about calls to the Create method.
		Create []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Instance is the instance argument value.
			Instance migration.Instance
		}
		// CreateOverrides holds details about calls to the CreateOverrides method.
		CreateOverrides []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Overrides is the overrides argument value.
			Overrides migration.InstanceOverride
		}
		// DeleteByUUID holds details about calls to the DeleteByUUID method.
		DeleteByUUID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ID is the id argument value.
			ID uuid.UUID
		}
		// DeleteOverridesByUUID holds details about calls to the DeleteOverridesByUUID method.
		DeleteOverridesByUUID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ID is the id argument value.
			ID uuid.UUID
		}
		// GetAll holds details about calls to the GetAll method.
		GetAll []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetAllByBatch holds details about calls to the GetAllByBatch method.
		GetAllByBatch []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Batch is the batch argument value.
			Batch string
		}
		// GetAllByBatchAndState holds details about calls to the GetAllByBatchAndState method.
		GetAllByBatchAndState []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Batch is the batch argument value.
			Batch string
			// Status is the status argument value.
			Status api.MigrationStatusType
		}
		// GetAllBySource holds details about calls to the GetAllBySource method.
		GetAllBySource []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Source is the source argument value.
			Source string
		}
		// GetAllByState holds details about calls to the GetAllByState method.
		GetAllByState []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Status is the status argument value.
			Status []api.MigrationStatusType
		}
		// GetAllUUIDs holds details about calls to the GetAllUUIDs method.
		GetAllUUIDs []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetAllUnassigned holds details about calls to the GetAllUnassigned method.
		GetAllUnassigned []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetByUUID holds details about calls to the GetByUUID method.
		GetByUUID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ID is the id argument value.
			ID uuid.UUID
		}
		// GetOverridesByUUID holds details about calls to the GetOverridesByUUID method.
		GetOverridesByUUID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ID is the id argument value.
			ID uuid.UUID
		}
		// Update holds details about calls to the Update method.
		Update []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Instance is the instance argument value.
			Instance migration.Instance
		}
		// UpdateOverrides holds details about calls to the UpdateOverrides method.
		UpdateOverrides []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Overrides is the overrides argument value.
			Overrides migration.InstanceOverride
		}
	}
	lockCreate                sync.RWMutex
	lockCreateOverrides       sync.RWMutex
	lockDeleteByUUID          sync.RWMutex
	lockDeleteOverridesByUUID sync.RWMutex
	lockGetAll                sync.RWMutex
	lockGetAllByBatch         sync.RWMutex
	lockGetAllByBatchAndState sync.RWMutex
	lockGetAllBySource        sync.RWMutex
	lockGetAllByState         sync.RWMutex
	lockGetAllUUIDs           sync.RWMutex
	lockGetAllUnassigned      sync.RWMutex
	lockGetByUUID             sync.RWMutex
	lockGetOverridesByUUID    sync.RWMutex
	lockUpdate                sync.RWMutex
	lockUpdateOverrides       sync.RWMutex
}

// Create calls CreateFunc.
func (mock *InstanceRepoMock) Create(ctx context.Context, instance migration.Instance) (int64, error) {
	if mock.CreateFunc == nil {
		panic("InstanceRepoMock.CreateFunc: method is nil but InstanceRepo.Create was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		Instance migration.Instance
	}{
		Ctx:      ctx,
		Instance: instance,
	}
	mock.lockCreate.Lock()
	mock.calls.Create = append(mock.calls.Create, callInfo)
	mock.lockCreate.Unlock()
	return mock.CreateFunc(ctx, instance)
}

// CreateCalls gets all the calls that were made to Create.
// Check the length with:
//
//	len(mockedInstanceRepo.CreateCalls())
func (mock *InstanceRepoMock) CreateCalls() []struct {
	Ctx      context.Context
	Instance migration.Instance
} {
	var calls []struct {
		Ctx      context.Context
		Instance migration.Instance
	}
	mock.lockCreate.RLock()
	calls = mock.calls.Create
	mock.lockCreate.RUnlock()
	return calls
}

// CreateOverrides calls CreateOverridesFunc.
func (mock *InstanceRepoMock) CreateOverrides(ctx context.Context, overrides migration.InstanceOverride) (int64, error) {
	if mock.CreateOverridesFunc == nil {
		panic("InstanceRepoMock.CreateOverridesFunc: method is nil but InstanceRepo.CreateOverrides was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		Overrides migration.InstanceOverride
	}{
		Ctx:       ctx,
		Overrides: overrides,
	}
	mock.lockCreateOverrides.Lock()
	mock.calls.CreateOverrides = append(mock.calls.CreateOverrides, callInfo)
	mock.lockCreateOverrides.Unlock()
	return mock.CreateOverridesFunc(ctx, overrides)
}

// CreateOverridesCalls gets all the calls that were made to CreateOverrides.
// Check the length with:
//
//	len(mockedInstanceRepo.CreateOverridesCalls())
func (mock *InstanceRepoMock) CreateOverridesCalls() []struct {
	Ctx       context.Context
	Overrides migration.InstanceOverride
} {
	var calls []struct {
		Ctx       context.Context
		Overrides migration.InstanceOverride
	}
	mock.lockCreateOverrides.RLock()
	calls = mock.calls.CreateOverrides
	mock.lockCreateOverrides.RUnlock()
	return calls
}

// DeleteByUUID calls DeleteByUUIDFunc.
func (mock *InstanceRepoMock) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	if mock.DeleteByUUIDFunc == nil {
		panic("InstanceRepoMock.DeleteByUUIDFunc: method is nil but InstanceRepo.DeleteByUUID was just called")
	}
	callInfo := struct {
		Ctx context.Context
		ID  uuid.UUID
	}{
		Ctx: ctx,
		ID:  id,
	}
	mock.lockDeleteByUUID.Lock()
	mock.calls.DeleteByUUID = append(mock.calls.DeleteByUUID, callInfo)
	mock.lockDeleteByUUID.Unlock()
	return mock.DeleteByUUIDFunc(ctx, id)
}

// DeleteByUUIDCalls gets all the calls that were made to DeleteByUUID.
// Check the length with:
//
//	len(mockedInstanceRepo.DeleteByUUIDCalls())
func (mock *InstanceRepoMock) DeleteByUUIDCalls() []struct {
	Ctx context.Context
	ID  uuid.UUID
} {
	var calls []struct {
		Ctx context.Context
		ID  uuid.UUID
	}
	mock.lockDeleteByUUID.RLock()
	calls = mock.calls.DeleteByUUID
	mock.lockDeleteByUUID.RUnlock()
	return calls
}

// DeleteOverridesByUUID calls DeleteOverridesByUUIDFunc.
func (mock *InstanceRepoMock) DeleteOverridesByUUID(ctx context.Context, id uuid.UUID) error {
	if mock.DeleteOverridesByUUIDFunc == nil {
		panic("InstanceRepoMock.DeleteOverridesByUUIDFunc: method is nil but InstanceRepo.DeleteOverridesByUUID was just called")
	}
	callInfo := struct {
		Ctx context.Context
		ID  uuid.UUID
	}{
		Ctx: ctx,
		ID:  id,
	}
	mock.lockDeleteOverridesByUUID.Lock()
	mock.calls.DeleteOverridesByUUID = append(mock.calls.DeleteOverridesByUUID, callInfo)
	mock.lockDeleteOverridesByUUID.Unlock()
	return mock.DeleteOverridesByUUIDFunc(ctx, id)
}

// DeleteOverridesByUUIDCalls gets all the calls that were made to DeleteOverridesByUUID.
// Check the length with:
//
//	len(mockedInstanceRepo.DeleteOverridesByUUIDCalls())
func (mock *InstanceRepoMock) DeleteOverridesByUUIDCalls() []struct {
	Ctx context.Context
	ID  uuid.UUID
} {
	var calls []struct {
		Ctx context.Context
		ID  uuid.UUID
	}
	mock.lockDeleteOverridesByUUID.RLock()
	calls = mock.calls.DeleteOverridesByUUID
	mock.lockDeleteOverridesByUUID.RUnlock()
	return calls
}

// GetAll calls GetAllFunc.
func (mock *InstanceRepoMock) GetAll(ctx context.Context) (migration.Instances, error) {
	if mock.GetAllFunc == nil {
		panic("InstanceRepoMock.GetAllFunc: method is nil but InstanceRepo.GetAll was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetAll.Lock()
	mock.calls.GetAll = append(mock.calls.GetAll, callInfo)
	mock.lockGetAll.Unlock()
	return mock.GetAllFunc(ctx)
}

// GetAllCalls gets all the calls that were made to GetAll.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllCalls())
func (mock *InstanceRepoMock) GetAllCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetAll.RLock()
	calls = mock.calls.GetAll
	mock.lockGetAll.RUnlock()
	return calls
}

// GetAllByBatch calls GetAllByBatchFunc.
func (mock *InstanceRepoMock) GetAllByBatch(ctx context.Context, batch string) (migration.Instances, error) {
	if mock.GetAllByBatchFunc == nil {
		panic("InstanceRepoMock.GetAllByBatchFunc: method is nil but InstanceRepo.GetAllByBatch was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Batch string
	}{
		Ctx:   ctx,
		Batch: batch,
	}
	mock.lockGetAllByBatch.Lock()
	mock.calls.GetAllByBatch = append(mock.calls.GetAllByBatch, callInfo)
	mock.lockGetAllByBatch.Unlock()
	return mock.GetAllByBatchFunc(ctx, batch)
}

// GetAllByBatchCalls gets all the calls that were made to GetAllByBatch.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllByBatchCalls())
func (mock *InstanceRepoMock) GetAllByBatchCalls() []struct {
	Ctx   context.Context
	Batch string
} {
	var calls []struct {
		Ctx   context.Context
		Batch string
	}
	mock.lockGetAllByBatch.RLock()
	calls = mock.calls.GetAllByBatch
	mock.lockGetAllByBatch.RUnlock()
	return calls
}

// GetAllByBatchAndState calls GetAllByBatchAndStateFunc.
func (mock *InstanceRepoMock) GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType) (migration.Instances, error) {
	if mock.GetAllByBatchAndStateFunc == nil {
		panic("InstanceRepoMock.GetAllByBatchAndStateFunc: method is nil but InstanceRepo.GetAllByBatchAndState was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Batch  string
		Status api.MigrationStatusType
	}{
		Ctx:    ctx,
		Batch:  batch,
		Status: status,
	}
	mock.lockGetAllByBatchAndState.Lock()
	mock.calls.GetAllByBatchAndState = append(mock.calls.GetAllByBatchAndState, callInfo)
	mock.lockGetAllByBatchAndState.Unlock()
	return mock.GetAllByBatchAndStateFunc(ctx, batch, status)
}

// GetAllByBatchAndStateCalls gets all the calls that were made to GetAllByBatchAndState.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllByBatchAndStateCalls())
func (mock *InstanceRepoMock) GetAllByBatchAndStateCalls() []struct {
	Ctx    context.Context
	Batch  string
	Status api.MigrationStatusType
} {
	var calls []struct {
		Ctx    context.Context
		Batch  string
		Status api.MigrationStatusType
	}
	mock.lockGetAllByBatchAndState.RLock()
	calls = mock.calls.GetAllByBatchAndState
	mock.lockGetAllByBatchAndState.RUnlock()
	return calls
}

// GetAllBySource calls GetAllBySourceFunc.
func (mock *InstanceRepoMock) GetAllBySource(ctx context.Context, source string) (migration.Instances, error) {
	if mock.GetAllBySourceFunc == nil {
		panic("InstanceRepoMock.GetAllBySourceFunc: method is nil but InstanceRepo.GetAllBySource was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Source string
	}{
		Ctx:    ctx,
		Source: source,
	}
	mock.lockGetAllBySource.Lock()
	mock.calls.GetAllBySource = append(mock.calls.GetAllBySource, callInfo)
	mock.lockGetAllBySource.Unlock()
	return mock.GetAllBySourceFunc(ctx, source)
}

// GetAllBySourceCalls gets all the calls that were made to GetAllBySource.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllBySourceCalls())
func (mock *InstanceRepoMock) GetAllBySourceCalls() []struct {
	Ctx    context.Context
	Source string
} {
	var calls []struct {
		Ctx    context.Context
		Source string
	}
	mock.lockGetAllBySource.RLock()
	calls = mock.calls.GetAllBySource
	mock.lockGetAllBySource.RUnlock()
	return calls
}

// GetAllByState calls GetAllByStateFunc.
func (mock *InstanceRepoMock) GetAllByState(ctx context.Context, status ...api.MigrationStatusType) (migration.Instances, error) {
	if mock.GetAllByStateFunc == nil {
		panic("InstanceRepoMock.GetAllByStateFunc: method is nil but InstanceRepo.GetAllByState was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Status []api.MigrationStatusType
	}{
		Ctx:    ctx,
		Status: status,
	}
	mock.lockGetAllByState.Lock()
	mock.calls.GetAllByState = append(mock.calls.GetAllByState, callInfo)
	mock.lockGetAllByState.Unlock()
	return mock.GetAllByStateFunc(ctx, status...)
}

// GetAllByStateCalls gets all the calls that were made to GetAllByState.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllByStateCalls())
func (mock *InstanceRepoMock) GetAllByStateCalls() []struct {
	Ctx    context.Context
	Status []api.MigrationStatusType
} {
	var calls []struct {
		Ctx    context.Context
		Status []api.MigrationStatusType
	}
	mock.lockGetAllByState.RLock()
	calls = mock.calls.GetAllByState
	mock.lockGetAllByState.RUnlock()
	return calls
}

// GetAllUUIDs calls GetAllUUIDsFunc.
func (mock *InstanceRepoMock) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	if mock.GetAllUUIDsFunc == nil {
		panic("InstanceRepoMock.GetAllUUIDsFunc: method is nil but InstanceRepo.GetAllUUIDs was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetAllUUIDs.Lock()
	mock.calls.GetAllUUIDs = append(mock.calls.GetAllUUIDs, callInfo)
	mock.lockGetAllUUIDs.Unlock()
	return mock.GetAllUUIDsFunc(ctx)
}

// GetAllUUIDsCalls gets all the calls that were made to GetAllUUIDs.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllUUIDsCalls())
func (mock *InstanceRepoMock) GetAllUUIDsCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetAllUUIDs.RLock()
	calls = mock.calls.GetAllUUIDs
	mock.lockGetAllUUIDs.RUnlock()
	return calls
}

// GetAllUnassigned calls GetAllUnassignedFunc.
func (mock *InstanceRepoMock) GetAllUnassigned(ctx context.Context) (migration.Instances, error) {
	if mock.GetAllUnassignedFunc == nil {
		panic("InstanceRepoMock.GetAllUnassignedFunc: method is nil but InstanceRepo.GetAllUnassigned was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetAllUnassigned.Lock()
	mock.calls.GetAllUnassigned = append(mock.calls.GetAllUnassigned, callInfo)
	mock.lockGetAllUnassigned.Unlock()
	return mock.GetAllUnassignedFunc(ctx)
}

// GetAllUnassignedCalls gets all the calls that were made to GetAllUnassigned.
// Check the length with:
//
//	len(mockedInstanceRepo.GetAllUnassignedCalls())
func (mock *InstanceRepoMock) GetAllUnassignedCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetAllUnassigned.RLock()
	calls = mock.calls.GetAllUnassigned
	mock.lockGetAllUnassigned.RUnlock()
	return calls
}

// GetByUUID calls GetByUUIDFunc.
func (mock *InstanceRepoMock) GetByUUID(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
	if mock.GetByUUIDFunc == nil {
		panic("InstanceRepoMock.GetByUUIDFunc: method is nil but InstanceRepo.GetByUUID was just called")
	}
	callInfo := struct {
		Ctx context.Context
		ID  uuid.UUID
	}{
		Ctx: ctx,
		ID:  id,
	}
	mock.lockGetByUUID.Lock()
	mock.calls.GetByUUID = append(mock.calls.GetByUUID, callInfo)
	mock.lockGetByUUID.Unlock()
	return mock.GetByUUIDFunc(ctx, id)
}

// GetByUUIDCalls gets all the calls that were made to GetByUUID.
// Check the length with:
//
//	len(mockedInstanceRepo.GetByUUIDCalls())
func (mock *InstanceRepoMock) GetByUUIDCalls() []struct {
	Ctx context.Context
	ID  uuid.UUID
} {
	var calls []struct {
		Ctx context.Context
		ID  uuid.UUID
	}
	mock.lockGetByUUID.RLock()
	calls = mock.calls.GetByUUID
	mock.lockGetByUUID.RUnlock()
	return calls
}

// GetOverridesByUUID calls GetOverridesByUUIDFunc.
func (mock *InstanceRepoMock) GetOverridesByUUID(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
	if mock.GetOverridesByUUIDFunc == nil {
		panic("InstanceRepoMock.GetOverridesByUUIDFunc: method is nil but InstanceRepo.GetOverridesByUUID was just called")
	}
	callInfo := struct {
		Ctx context.Context
		ID  uuid.UUID
	}{
		Ctx: ctx,
		ID:  id,
	}
	mock.lockGetOverridesByUUID.Lock()
	mock.calls.GetOverridesByUUID = append(mock.calls.GetOverridesByUUID, callInfo)
	mock.lockGetOverridesByUUID.Unlock()
	return mock.GetOverridesByUUIDFunc(ctx, id)
}

// GetOverridesByUUIDCalls gets all the calls that were made to GetOverridesByUUID.
// Check the length with:
//
//	len(mockedInstanceRepo.GetOverridesByUUIDCalls())
func (mock *InstanceRepoMock) GetOverridesByUUIDCalls() []struct {
	Ctx context.Context
	ID  uuid.UUID
} {
	var calls []struct {
		Ctx context.Context
		ID  uuid.UUID
	}
	mock.lockGetOverridesByUUID.RLock()
	calls = mock.calls.GetOverridesByUUID
	mock.lockGetOverridesByUUID.RUnlock()
	return calls
}

// Update calls UpdateFunc.
func (mock *InstanceRepoMock) Update(ctx context.Context, instance migration.Instance) error {
	if mock.UpdateFunc == nil {
		panic("InstanceRepoMock.UpdateFunc: method is nil but InstanceRepo.Update was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		Instance migration.Instance
	}{
		Ctx:      ctx,
		Instance: instance,
	}
	mock.lockUpdate.Lock()
	mock.calls.Update = append(mock.calls.Update, callInfo)
	mock.lockUpdate.Unlock()
	return mock.UpdateFunc(ctx, instance)
}

// UpdateCalls gets all the calls that were made to Update.
// Check the length with:
//
//	len(mockedInstanceRepo.UpdateCalls())
func (mock *InstanceRepoMock) UpdateCalls() []struct {
	Ctx      context.Context
	Instance migration.Instance
} {
	var calls []struct {
		Ctx      context.Context
		Instance migration.Instance
	}
	mock.lockUpdate.RLock()
	calls = mock.calls.Update
	mock.lockUpdate.RUnlock()
	return calls
}

// UpdateOverrides calls UpdateOverridesFunc.
func (mock *InstanceRepoMock) UpdateOverrides(ctx context.Context, overrides migration.InstanceOverride) error {
	if mock.UpdateOverridesFunc == nil {
		panic("InstanceRepoMock.UpdateOverridesFunc: method is nil but InstanceRepo.UpdateOverrides was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		Overrides migration.InstanceOverride
	}{
		Ctx:       ctx,
		Overrides: overrides,
	}
	mock.lockUpdateOverrides.Lock()
	mock.calls.UpdateOverrides = append(mock.calls.UpdateOverrides, callInfo)
	mock.lockUpdateOverrides.Unlock()
	return mock.UpdateOverridesFunc(ctx, overrides)
}

// UpdateOverridesCalls gets all the calls that were made to UpdateOverrides.
// Check the length with:
//
//	len(mockedInstanceRepo.UpdateOverridesCalls())
func (mock *InstanceRepoMock) UpdateOverridesCalls() []struct {
	Ctx       context.Context
	Overrides migration.InstanceOverride
} {
	var calls []struct {
		Ctx       context.Context
		Overrides migration.InstanceOverride
	}
	mock.lockUpdateOverrides.RLock()
	calls = mock.calls.UpdateOverrides
	mock.lockUpdateOverrides.RUnlock()
	return calls
}
