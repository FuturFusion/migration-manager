package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddBatch(tx *sql.Tx, b batch.Batch) error {
	internalBatch, ok := b.(*batch.InternalBatch)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalBatch?")
	}

	// Add batch to the database.
	q := `INSERT INTO batches (name,status,includeregex,excluderegex,migrationwindowstart,migrationwindowend) VALUES(?,?,?,?,?,?)`

	marshalledMigrationWindowStart, err := internalBatch.MigrationWindowStart.MarshalText()
	if err != nil {
		return err
	}
	marshalledMigrationWindowEnd, err := internalBatch.MigrationWindowEnd.MarshalText()
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, internalBatch.Name, internalBatch.Status, internalBatch.IncludeRegex, internalBatch.ExcludeRegex, marshalledMigrationWindowStart, marshalledMigrationWindowEnd)
	if err != nil {
		return err
	}

	// Set the new ID assigned to the batch.
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return err
	}
	internalBatch.DatabaseID = int(lastInsertId)

	return nil
}

func (n *Node) GetBatch(tx *sql.Tx, name string) (batch.Batch, error) {
	ret, err := n.getBatchesHelper(tx, name)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No batch exists with name '%s'", name)
	}

	return ret[0], nil
}

func (n *Node) GetAllBatches(tx *sql.Tx) ([]batch.Batch, error) {
	return n.getBatchesHelper(tx, "")
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
	for _, i := range instances {
		instance, err := n.GetInstance(tx, i)
		if err != nil {
			return err
		}

		if instance.IsMigrating() {
			return fmt.Errorf("Cannot delete batch '%s': At least one assigned instance is in a migration phase", name)
		}

		q := `UPDATE instances SET batchid=?,migrationstatus=?,migrationstatusstring=? WHERE uuid=?`
		var status api.MigrationStatusType = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
		_, err = tx.Exec(q, internal.INVALID_DATABASE_ID, status, status.String(), instance.GetUUID())
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
	q = `UPDATE batches SET name=?,status=?,includeregex=?,excluderegex=?,migrationwindowstart=?,migrationwindowend=? WHERE id=?`

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
	result, err := tx.Exec(q, internalBatch.Name, internalBatch.Status, internalBatch.IncludeRegex, internalBatch.ExcludeRegex, marshalledMigrationWindowStart, marshalledMigrationWindowEnd, internalBatch.DatabaseID)
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

func (n *Node) getBatchesHelper(tx *sql.Tx, name string) ([]batch.Batch, error) {
	ret := []batch.Batch{}

	// Get all batches in the database.
	q := `SELECT id,name,status,includeregex,excluderegex,migrationwindowstart,migrationwindowend FROM batches`
	var rows *sql.Rows
	var err error
	if name != "" {
		q += ` WHERE name=?`
		rows, err = tx.Query(q, name)
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

		err := rows.Scan(&newBatch.DatabaseID, &newBatch.Name, &newBatch.Status, &newBatch.IncludeRegex, &newBatch.ExcludeRegex, &marshalledMigrationWindowStart, &marshalledMigrationWindowEnd)
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

func (n *Node) GetAllInstancesForBatchID(tx *sql.Tx, id int) ([]uuid.UUID, error) {
	ret := []uuid.UUID{}
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
		ret = append(ret, instanceUUID)
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
		instance, err := n.GetInstance(tx, i)
		if err != nil {
			return err
		}

		if !b.InstanceMatchesCriteria(instance) {
			if instance.CanBeModified() {
				q := `UPDATE instances SET batchid=?,migrationstatus=?,migrationstatusstring=? WHERE uuid=?`
				var status api.MigrationStatusType = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
				_, err := tx.Exec(q, internal.INVALID_DATABASE_ID, status, status.String(), instance.GetUUID())
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
		instance, err := n.GetInstance(tx, i)
		if err != nil {
			return err
		}

		if b.InstanceMatchesCriteria(instance) {
			if instance.CanBeModified() {
				q := `UPDATE instances SET batchid=?,migrationstatus=?,migrationstatusstring=? WHERE uuid=?`
				var status api.MigrationStatusType = api.MIGRATIONSTATUS_ASSIGNED_BATCH
				_, err := tx.Exec(q, batchID, status, status.String(), instance.GetUUID())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
