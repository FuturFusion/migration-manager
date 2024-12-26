package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddInstanceOverride(tx *sql.Tx, override api.InstanceOverride) error {
	// Add the override to the database.
	q := `INSERT INTO instance_overrides (uuid,last_update,comment,number_cpus,memory_in_bytes) VALUES(?,?,?,?,?)`

	marshalledLastUpdate, err := override.LastUpdate.MarshalText()
	if err != nil {
		return err
	}

	_, err = tx.Exec(q, override.UUID, marshalledLastUpdate, override.Comment, override.NumberCPUs, override.MemoryInBytes)

	return mapDBError(err)
}

func (n *Node) GetInstanceOverride(tx *sql.Tx, UUID uuid.UUID) (api.InstanceOverride, error) {
	ret := api.InstanceOverride{}

	// Get the override from the database.
	q := `SELECT last_update,comment,number_cpus,memory_in_bytes FROM instance_overrides WHERE uuid=?`
	row := tx.QueryRow(q, UUID)

	marshalledLastUpdate := ""

	err := row.Scan(&marshalledLastUpdate, &ret.Comment, &ret.NumberCPUs, &ret.MemoryInBytes)
	if err != nil {
		return ret, mapDBError(err)
	}

	err = ret.LastUpdate.UnmarshalText([]byte(marshalledLastUpdate))
	if err != nil {
		return ret, err
	}

	ret.UUID = UUID

	return ret, nil
}

func (n *Node) DeleteInstanceOverride(tx *sql.Tx, UUID uuid.UUID) error {
	// Don't allow deletion if the corresponding instance is in a migration phase.
	i, err := n.GetInstance(tx, UUID)
	if err != nil {
		return err
	}

	if i.GetBatchID() != nil || i.IsMigrating() {
		return fmt.Errorf("Cannot delete override for instance '%s': Either assigned to a batch or currently migrating", i.GetInventoryPath())
	}

	// Delete the override from the database.
	q := `DELETE FROM instance_overrides WHERE uuid=?`
	result, err := tx.Exec(q, UUID)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Instance override with UUID '%s' doesn't exist, can't delete", UUID)
	}

	return nil
}

func (n *Node) UpdateInstanceOverride(tx *sql.Tx, override api.InstanceOverride) error {
	// Don't allow updates if the corresponding instance has been assigned to a batch.
	q := `SELECT batch_id,inventory_path FROM instances WHERE uuid=?`
	row := tx.QueryRow(q, override.UUID)

	var batchID *int
	inventoryPath := ""
	err := row.Scan(&batchID, &inventoryPath)
	if err != nil {
		return mapDBError(err)
	}

	if batchID != nil {
		q = `SELECT name FROM batches WHERE id=?`
		row = tx.QueryRow(q, batchID)

		batchName := ""
		err := row.Scan(&batchName)
		if err != nil {
			return mapDBError(err)
		}

		return fmt.Errorf("Cannot update override for instance '%s' while assigned to batch '%s'", inventoryPath, batchName)
	}

	// Update override in the database.
	q = `UPDATE instance_overrides SET last_update=?,comment=?,number_cpus=?,memory_in_bytes=? WHERE uuid=?`

	marshalledLastUpdate, err := override.LastUpdate.MarshalText()
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, marshalledLastUpdate, override.Comment, override.NumberCPUs, override.MemoryInBytes, override.UUID)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Instance with UUID '%s' doesn't exist, can't update", override.UUID)
	}

	return nil
}
