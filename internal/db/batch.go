package db

import (
	"database/sql"
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
	q := `INSERT INTO batches (name,status,statusstring,storagepool,includeregex,excluderegex,migrationwindowstart,migrationwindowend,defaultnetwork) VALUES(?,?,?,?,?,?,?,?,?)`

	marshalledMigrationWindowStart, err := internalBatch.MigrationWindowStart.MarshalText()
	if err != nil {
		return err
	}
	marshalledMigrationWindowEnd, err := internalBatch.MigrationWindowEnd.MarshalText()
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, internalBatch.Name, internalBatch.Status, internalBatch.StatusString, internalBatch.StoragePool, internalBatch.IncludeRegex, internalBatch.ExcludeRegex, marshalledMigrationWindowStart, marshalledMigrationWindowEnd, internalBatch.DefaultNetwork)
	if err != nil {
		return err
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

		q := `UPDATE instances SET batchid=?,migrationstatus=?,migrationstatusstring=? WHERE uuid=?`
		_, err = tx.Exec(q, internal.INVALID_DATABASE_ID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), inst.GetUUID())
		if err != nil {
			return err
		}
	}

	// Delete the batch from the database.
	q := `DELETE FROM batches WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return err
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
		return err
	}

	dbBatch, err := n.GetBatch(tx, origName)
	if err != nil {
		return err
	}
	if !dbBatch.CanBeModified() {
		return fmt.Errorf("Cannot update batch '%s': Currently in a migration phase", b.GetName())
	}

	// Update batch in the database.
	q = `UPDATE batches SET name=?,status=?,statusstring=?,storagepool=?,includeregex=?,excluderegex=?,migrationwindowstart=?,migrationwindowend=?,defaultnetwork=? WHERE id=?`

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
	result, err := tx.Exec(q, internalBatch.Name, internalBatch.Status, internalBatch.StatusString, internalBatch.StoragePool, internalBatch.IncludeRegex, internalBatch.ExcludeRegex, marshalledMigrationWindowStart, marshalledMigrationWindowEnd, internalBatch.DefaultNetwork, internalBatch.DatabaseID)
	if err != nil {
		return err
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
	q := `SELECT id,name,status,statusstring,storagepool,includeregex,excluderegex,migrationwindowstart,migrationwindowend,defaultnetwork FROM batches`
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
		return ret, err
	}

	for rows.Next() {
		newBatch := &batch.InternalBatch{}
		marshalledMigrationWindowStart := ""
		marshalledMigrationWindowEnd := ""

		err := rows.Scan(&newBatch.DatabaseID, &newBatch.Name, &newBatch.Status, &newBatch.StatusString, &newBatch.StoragePool, &newBatch.IncludeRegex, &newBatch.ExcludeRegex, &marshalledMigrationWindowStart, &marshalledMigrationWindowEnd, &newBatch.DefaultNetwork)
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

	return ret, nil
}

func (n *Node) GetAllInstancesForBatchID(tx *sql.Tx, id int) ([]instance.Instance, error) {
	ret := []instance.Instance{}
	q := `SELECT uuid FROM instances WHERE batchid=?`
	rows, err := tx.Query(q, id)
	if err != nil {
		return nil, err
	}

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

	// Check if each existing instance should still be assigned to this batch.
	for _, i := range instances {
		if !b.InstanceMatchesCriteria(i) {
			if i.CanBeModified() {
				q := `UPDATE instances SET batchid=?,migrationstatus=?,migrationstatusstring=? WHERE uuid=?`
				_, err := tx.Exec(q, internal.INVALID_DATABASE_ID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), i.GetUUID())
				if err != nil {
					return err
				}
			}
		}
	}

	// Get a list of all unassigned instances.
	instances, err = n.GetAllInstancesForBatchID(tx, internal.INVALID_DATABASE_ID)
	if err != nil {
		return err
	}

	// Check if any unassigned instances should be assigned to this batch.
	for _, i := range instances {
		if b.InstanceMatchesCriteria(i) {
			if i.CanBeModified() {
				q := `UPDATE instances SET batchid=?,migrationstatus=?,migrationstatusstring=? WHERE uuid=?`
				_, err := tx.Exec(q, batchID, api.MIGRATIONSTATUS_ASSIGNED_BATCH, api.MIGRATIONSTATUS_ASSIGNED_BATCH.String(), i.GetUUID())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
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
	q := `UPDATE batches SET status=?,statusstring=? WHERE id=?`
	_, err := tx.Exec(q, status, statusString, id)

	return err
}
