package entities

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
)

// Code generation directives.
//
//generate-database:mapper target instance_batch.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e instance_batch objects
//generate-database:mapper stmt -e instance_batch objects-by-BatchID
//generate-database:mapper stmt -e instance_batch objects-by-InstanceID
//generate-database:mapper stmt -e instance_batch create
//generate-database:mapper stmt -e instance_batch delete-by-InstanceID
//generate-database:mapper stmt -e instance_batch delete-by-InstanceID-and-BatchID
//
//generate-database:mapper method -e instance_batch GetMany struct=Batch
//generate-database:mapper method -e instance_batch GetMany struct=Instance
//generate-database:mapper method -e instance_batch Create struct=Instance
//generate-database:mapper method -e instance_batch DeleteMany struct=Instance

type InstanceBatch struct {
	BatchID    int64 `db:"primary=yes"`
	InstanceID int64 `db:"primary=yes"`
}

type InstanceBatchFilter struct {
	BatchID    *int64
	InstanceID *int64
}

func GetAssignedInstances(ctx context.Context, tx dbtx) (migration.Instances, error) {
	stmt := fmt.Sprintf(`SELECT %s
FROM instances
JOIN instances_batches ON instances_batches.instance_id = instances.id
JOIN sources ON instances.source_id = sources.id
ORDER BY instances.uuid
`, instanceColumns())

	return getInstancesRaw(ctx, tx, stmt)
}

func GetInstancesByBatch(ctx context.Context, tx dbtx, batchName *string) (migration.Instances, error) {
	if batchName != nil {
		stmt := fmt.Sprintf(`SELECT %s
FROM instances
JOIN instances_batches ON instances_batches.instance_id = instances.id
JOIN batches ON instances_batches.batch_id = batches.id
JOIN sources ON instances.source_id = sources.id
WHERE batches.name = ?
ORDER BY instances.uuid
`, instanceColumns())

		return getInstancesRaw(ctx, tx, stmt, *batchName)
	}

	stmt := fmt.Sprintf(`SELECT %s
FROM instances
LEFT JOIN instances_batches ON instances_batches.instance_id = instances.id
JOIN sources ON instances.source_id = sources.id
WHERE instances_batches.instance_id IS NULL
ORDER BY instances.uuid
`, instanceColumns())

	return getInstancesRaw(ctx, tx, stmt)
}

func GetBatchesByInstance(ctx context.Context, tx dbtx, instanceUUID *uuid.UUID) (migration.Batches, error) {
	if instanceUUID != nil {
		stmt := fmt.Sprintf(`SELECT %s
FROM batches
JOIN instances_batches ON instances_batches.batch_id = batches.id
JOIN instances ON instances_batches.instance_id = instances.id
JOIN sources ON instances.source_id = sources.id
WHERE instances.uuid = ?
ORDER BY batches.name
`, batchColumns())

		return getBatchesRaw(ctx, tx, stmt, *instanceUUID)
	}

	stmt := fmt.Sprintf(`SELECT %s
FROM batches
LEFT_JOIN instances_batches ON instances_batches.batch_id = batches.id
JOIN sources ON instances.source_id = sources.id
WHERE instances_batches.batch_id IS NULL
ORDER BY batches.name
`, batchColumns())

	return getBatchesRaw(ctx, tx, stmt)
}

// DeleteInstanceBatch deletes the instance_batch matching the given key parameters.
func DeleteInstanceBatch(ctx context.Context, db tx, instanceID int, batchID int) (_err error) {
	defer func() {
		_err = mapErr(_err, "Instance_batch")
	}()

	stmt, err := Stmt(db, instanceBatchDeleteByInstanceIDAndBatchID)
	if err != nil {
		return fmt.Errorf("Failed to get \"instanceBatchDeleteByInstanceID\" prepared statement: %w", err)
	}

	result, err := stmt.Exec(instanceID, batchID)
	if err != nil {
		return fmt.Errorf("Delete \"instances_batches\" entry failed: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Fetch affected rows: %w", err)
	}

	if n == 0 {
		return ErrNotFound
	} else if n > 1 {
		return fmt.Errorf("Query deleted %d Instance rows instead of 1", n)
	}

	return nil
}
