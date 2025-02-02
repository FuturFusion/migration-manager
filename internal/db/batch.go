package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddBatch(tx *sql.Tx, b batch.Batch) error {
	internalBatch, ok := b.(*batch.InternalBatch)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalBatch?")
	}

	// Add batch to the database.
	q := `INSERT INTO batches (name,target_id,target_project,status,status_string,storage_pool,include_expression,migration_window_start,migration_window_end) VALUES(?,?,?,?,?,?,?,?,?)`

	marshalledMigrationWindowStart, err := internalBatch.MigrationWindowStart.MarshalText()
	if err != nil {
		return err
	}

	marshalledMigrationWindowEnd, err := internalBatch.MigrationWindowEnd.MarshalText()
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, internalBatch.Name, internalBatch.TargetID, internalBatch.TargetProject, internalBatch.Status, internalBatch.StatusString, internalBatch.StoragePool, internalBatch.IncludeExpression, marshalledMigrationWindowStart, marshalledMigrationWindowEnd)
	if err != nil {
		return mapDBError(err)
	}

	// Set the new ID assigned to the batch.
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	internalBatch.DatabaseID = int(lastInsertID)

	return nil
}

func (n *Node) GetBatch(tx *sql.Tx, name string) (batch.Batch, error) {
	ret, err := n.getBatchesHelper(tx, name, internal.INVALID_DATABASE_ID)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No batch exists with name '%s'", name)
	}

	return ret[0], nil
}

func (n *Node) GetBatchByID(tx *sql.Tx, id int) (batch.Batch, error) {
	ret, err := n.getBatchesHelper(tx, "", id)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No batch exists with ID '%d'", id)
	}

	return ret[0], nil
}

func (n *Node) GetAllBatches(tx *sql.Tx) ([]batch.Batch, error) {
	return n.getBatchesHelper(tx, "", internal.INVALID_DATABASE_ID)
}

func (n *Node) DeleteBatch(tx *sql.Tx, name string) error {
	// Don't allow deletion if the batch is in a migration phase.
	dbBatch, err := n.GetBatch(tx, name)
	if err != nil {
		return err
	}

	if !dbBatch.CanBeModified() {
		return fmt.Errorf("Cannot delete batch '%s': Currently in a migration phase", name)
	}

	// Get a list of any instances currently assigned to this batch.
	batchID, err := dbBatch.GetDatabaseID()
	if err != nil {
		return err
	}

	instances, err := n.GetAllInstancesForBatchID(tx, batchID)
	if err != nil {
		return err
	}

	// Verify all instances for this batch aren't in a migration phase and remove their association with this batch.
	for _, inst := range instances {
		if inst.IsMigrating() {
			return fmt.Errorf("Cannot delete batch '%s': At least one assigned instance is in a migration phase", name)
		}

		q := `UPDATE instances SET batch_id=?,migration_status=?,migration_status_string=? WHERE uuid=?`
		_, err = tx.Exec(q, internal.INVALID_DATABASE_ID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), inst.GetUUID())
		if err != nil {
			return mapDBError(err)
		}
	}

	// Delete the batch from the database.
	q := `DELETE FROM batches WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Batch with name '%s' doesn't exist, can't delete", name)
	}

	return nil
}

func (n *Node) UpdateBatch(tx *sql.Tx, b batch.Batch) error {
	// Don't allow updates if the batch is in a migration phase.
	q := `SELECT name FROM batches WHERE id=?`
	id, err := b.GetDatabaseID()
	if err != nil {
		return err
	}

	row := tx.QueryRow(q, id)

	origName := ""
	err = row.Scan(&origName)
	if err != nil {
		return mapDBError(err)
	}

	dbBatch, err := n.GetBatch(tx, origName)
	if err != nil {
		return err
	}

	if !dbBatch.CanBeModified() {
		return fmt.Errorf("Cannot update batch '%s': Currently in a migration phase", b.GetName())
	}

	// Update batch in the database.
	q = `UPDATE batches SET name=?,target_id=?,target_project=?,status=?,status_string=?,storage_pool=?,include_expression=?,migration_window_start=?,migration_window_end=? WHERE id=?`

	internalBatch, ok := b.(*batch.InternalBatch)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalBatch?")
	}

	marshalledMigrationWindowStart, err := internalBatch.MigrationWindowStart.MarshalText()
	if err != nil {
		return err
	}

	marshalledMigrationWindowEnd, err := internalBatch.MigrationWindowEnd.MarshalText()
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, internalBatch.Name, internalBatch.TargetID, internalBatch.TargetProject, internalBatch.Status, internalBatch.StatusString, internalBatch.StoragePool, internalBatch.IncludeExpression, marshalledMigrationWindowStart, marshalledMigrationWindowEnd, internalBatch.DatabaseID)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Batch with ID %d doesn't exist, can't update", internalBatch.DatabaseID)
	}

	return nil
}

func (n *Node) getBatchesHelper(tx *sql.Tx, name string, id int) ([]batch.Batch, error) {
	ret := []batch.Batch{}

	// Get all batches in the database.
	q := `SELECT id,name,target_id,target_project,status,status_string,storage_pool,include_expression,migration_window_start,migration_window_end FROM batches`
	var rows *sql.Rows
	var err error
	if name != "" {
		q += ` WHERE name=?`
		rows, err = tx.Query(q, name)
	} else if id != internal.INVALID_DATABASE_ID {
		q += ` WHERE id=?`
		rows, err = tx.Query(q, id)
	} else {
		q += ` ORDER BY name`
		rows, err = tx.Query(q)
	}

	if err != nil {
		return nil, mapDBError(err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		newBatch := &batch.InternalBatch{}
		marshalledMigrationWindowStart := ""
		marshalledMigrationWindowEnd := ""

		err := rows.Scan(&newBatch.DatabaseID, &newBatch.Name, &newBatch.TargetID, &newBatch.TargetProject, &newBatch.Status, &newBatch.StatusString, &newBatch.StoragePool, &newBatch.IncludeExpression, &marshalledMigrationWindowStart, &marshalledMigrationWindowEnd)
		if err != nil {
			return nil, err
		}

		err = newBatch.MigrationWindowStart.UnmarshalText([]byte(marshalledMigrationWindowStart))
		if err != nil {
			return nil, err
		}

		err = newBatch.MigrationWindowEnd.UnmarshalText([]byte(marshalledMigrationWindowEnd))
		if err != nil {
			return nil, err
		}

		ret = append(ret, newBatch)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

func (n *Node) GetAllInstancesForBatchID(tx *sql.Tx, id int) ([]instance.Instance, error) {
	ret := []instance.Instance{}
	q := `SELECT uuid FROM instances WHERE batch_id=?`
	rows, err := tx.Query(q, id)
	if err != nil {
		return nil, mapDBError(err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		u := ""
		err := rows.Scan(&u)
		if err != nil {
			return nil, err
		}

		instanceUUID, err := uuid.Parse(u)
		if err != nil {
			return nil, err
		}

		i, err := n.GetInstance(tx, instanceUUID)
		if err != nil {
			return nil, err
		}

		ret = append(ret, i)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

func (n *Node) UpdateInstancesAssignedToBatch(tx *sql.Tx, b batch.Batch) error {
	// Get a list of any instances currently assigned to this batch.
	batchID, err := b.GetDatabaseID()
	if err != nil {
		return err
	}

	instances, err := n.GetAllInstancesForBatchID(tx, batchID)
	if err != nil {
		return err
	}

	// Update each instance for this batch.
	for _, i := range instances {
		// Check if the instance should still be assigned to this batch.
		if i.GetMigrationStatus() == api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
			continue
		}

		instWithDetails, err := n.getInstanceWithDetails(tx, i)
		if err != nil {
			return err
		}

		isMatch, err := b.InstanceMatchesCriteria(instWithDetails)
		if err != nil {
			return err
		}

		if !isMatch {
			if !i.IsMigrating() {
				q := `UPDATE instances SET batch_id=?,target_id=?,migration_status=?,migration_status_string=? WHERE uuid=?`
				_, err := tx.Exec(q, internal.INVALID_DATABASE_ID, internal.INVALID_DATABASE_ID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), i.GetUUID())
				if err != nil {
					return mapDBError(err)
				}
			}
		} else {
			// Ensure the target ID is synced from the batch to this instance.
			if !i.IsMigrating() {
				q := `UPDATE instances SET target_id=? WHERE uuid=?`
				_, err := tx.Exec(q, b.GetTargetID(), i.GetUUID())
				if err != nil {
					return mapDBError(err)
				}
			}
		}
	}

	// Get a list of all unassigned instances.
	instances, err = n.getAllUnassignedInstances(tx)
	if err != nil {
		return err
	}

	// Check if any unassigned instances should be assigned to this batch.
	for _, i := range instances {
		if i.GetMigrationStatus() == api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
			continue
		}

		instWithDetails, err := n.getInstanceWithDetails(tx, i)
		if err != nil {
			return err
		}

		isMatch, err := b.InstanceMatchesCriteria(instWithDetails)
		if err != nil {
			return err
		}

		if isMatch {
			if i.CanBeModified() {
				q := `UPDATE instances SET batch_id=?,target_id=?,migration_status=?,migration_status_string=? WHERE uuid=?`
				_, err := tx.Exec(q, batchID, b.GetTargetID(), api.MIGRATIONSTATUS_ASSIGNED_BATCH, api.MIGRATIONSTATUS_ASSIGNED_BATCH.String(), i.GetUUID())
				if err != nil {
					return mapDBError(err)
				}
			}
		}
	}

	return nil
}

func (n *Node) getAllUnassignedInstances(tx *sql.Tx) ([]instance.Instance, error) {
	ret := []instance.Instance{}
	q := `SELECT uuid FROM instances WHERE batch_id IS NULL`
	rows, err := tx.Query(q)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		u := ""
		err := rows.Scan(&u)
		if err != nil {
			return nil, err
		}

		instanceUUID, err := uuid.Parse(u)
		if err != nil {
			return nil, err
		}

		i, err := n.GetInstance(tx, instanceUUID)
		if err != nil {
			return nil, err
		}

		ret = append(ret, i)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

func (n *Node) getInstanceWithDetails(tx *sql.Tx, ii instance.Instance) (batch.InstanceWithDetails, error) {
	i, ok := ii.(*instance.InternalInstance)
	if !ok {
		return batch.InstanceWithDetails{}, errors.New("Invalid instance provided")
	}

	source, err := n.GetSourceByID(tx, i.SourceID)
	if err != nil {
		return batch.InstanceWithDetails{}, err
	}

	return batch.InstanceWithDetails{
		Name:              ii.GetName(),
		InventoryPath:     i.InventoryPath,
		Annotation:        i.Annotation,
		GuestToolsVersion: i.GuestToolsVersion,
		Architecture:      i.Architecture,
		HardwareVersion:   i.HardwareVersion,
		OS:                i.OS,
		OSVersion:         i.OSVersion,
		Devices:           i.Devices,
		Disks:             i.Disks,
		NICs:              i.NICs,
		Snapshots:         i.Snapshots,
		CPU:               i.CPU,
		Memory:            i.Memory,
		UseLegacyBios:     i.UseLegacyBios,
		SecureBootEnabled: i.SecureBootEnabled,
		TPMPresent:        i.TPMPresent,
		Source: batch.Source{
			Name:       source.Name,
			SourceType: source.SourceType.String(),
		},
		Overrides: i.Overrides,
	}, nil
}

func (n *Node) StartBatch(tx *sql.Tx, name string) error {
	// Get the batch to start.
	b, err := n.GetBatch(tx, name)
	if err != nil {
		return err
	}

	internalBatch, ok := b.(*batch.InternalBatch)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalBatch?")
	}

	// Ensure batch is in a state that is ready to start.
	if internalBatch.Status != api.BATCHSTATUS_DEFINED && internalBatch.Status != api.BATCHSTATUS_STOPPED && internalBatch.Status != api.BATCHSTATUS_ERROR {
		return fmt.Errorf("Cannot start batch '%s' in its current state '%s'", internalBatch.Name, internalBatch.Status)
	}

	// Move batch status to "ready".
	return n.UpdateBatchStatus(tx, internalBatch.DatabaseID, api.BATCHSTATUS_READY, api.BATCHSTATUS_READY.String())
}

func (n *Node) StopBatch(tx *sql.Tx, name string) error {
	// Get the batch to stop.
	b, err := n.GetBatch(tx, name)
	if err != nil {
		return err
	}

	internalBatch, ok := b.(*batch.InternalBatch)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalBatch?")
	}

	// Ensure batch is in a state that is ready to stop.
	if internalBatch.Status != api.BATCHSTATUS_READY && internalBatch.Status != api.BATCHSTATUS_QUEUED && internalBatch.Status != api.BATCHSTATUS_RUNNING {
		return fmt.Errorf("Cannot stop batch '%s' in its current state '%s'", internalBatch.Name, internalBatch.Status)
	}

	// Move batch status to "stopped".
	return n.UpdateBatchStatus(tx, internalBatch.DatabaseID, api.BATCHSTATUS_STOPPED, api.BATCHSTATUS_STOPPED.String())
}

func (n *Node) GetAllBatchesByState(tx *sql.Tx, status api.BatchStatusType) ([]batch.Batch, error) {
	ret := []batch.Batch{}

	batches, err := n.GetAllBatches(tx)
	if err != nil {
		return nil, err
	}

	for _, b := range batches {
		if b.GetStatus() == status {
			ret = append(ret, b)
		}
	}

	return ret, nil
}

func (n *Node) UpdateBatchStatus(tx *sql.Tx, id int, status api.BatchStatusType, statusString string) error {
	q := `UPDATE batches SET status=?,status_string=? WHERE id=?`
	_, err := tx.Exec(q, status, statusString, id)

	return mapDBError(err)
}
