package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddInstance(tx *sql.Tx, i instance.Instance) error {
	internalInstance, ok := i.(*instance.InternalInstance)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalInstance?")
	}

	// Add instance to the database.
	q := `INSERT INTO instances (uuid,inventory_path,migration_status,migration_status_string,last_update_from_source,source_id,target_id,batch_id,name,architecture,os,os_version,disks,nics,number_cpus,memory_in_bytes,use_legacy_bios,secure_boot_enabled,tpm_present,needs_disk_import) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	marshalledLastUpdateFromSource, err := internalInstance.LastUpdateFromSource.MarshalText()
	if err != nil {
		return err
	}

	marshalledDisks, err := json.Marshal(internalInstance.Disks)
	if err != nil {
		return err
	}

	marshalledNICs, err := json.Marshal(internalInstance.NICs)
	if err != nil {
		return err
	}

	_, err = tx.Exec(q, internalInstance.UUID, internalInstance.InventoryPath, internalInstance.MigrationStatus, internalInstance.MigrationStatusString, marshalledLastUpdateFromSource, internalInstance.SourceID, internalInstance.TargetID, internalInstance.BatchID, internalInstance.Name, internalInstance.Architecture, internalInstance.OS, internalInstance.OSVersion, marshalledDisks, marshalledNICs, internalInstance.NumberCPUs, internalInstance.MemoryInBytes, internalInstance.UseLegacyBios, internalInstance.SecureBootEnabled, internalInstance.TPMPresent, internalInstance.NeedsDiskImport)

	return err
}

func (n *Node) GetInstance(tx *sql.Tx, UUID uuid.UUID) (instance.Instance, error) {
	ret, err := n.getInstancesHelper(tx, UUID)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No instance exists with UUID '%s'", UUID)
	}

	return ret[0], nil
}

func (n *Node) GetAllInstances(tx *sql.Tx) ([]instance.Instance, error) {
	return n.getInstancesHelper(tx, [16]byte{})
}

func (n *Node) DeleteInstance(tx *sql.Tx, UUID uuid.UUID) error {
	// Don't allow deletion if the instance is in a migration phase.
	i, err := n.GetInstance(tx, UUID)
	if err != nil {
		return err
	}

	if i.GetBatchID() != internal.INVALID_DATABASE_ID || i.IsMigrating() {
		return fmt.Errorf("Cannot delete instance '%s': Either assigned to a batch or currently migrating", i.GetName())
	}

	// Delete any corresponding override first.
	q := `DELETE FROM instance_overrides WHERE uuid=?`
	_, err = tx.Exec(q, UUID)
	if err != nil {
		return err
	}

	// Delete the instance from the database.
	q = `DELETE FROM instances WHERE uuid=?`
	result, err := tx.Exec(q, UUID)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Instance with UUID '%s' doesn't exist, can't delete", UUID)
	}

	return nil
}

func (n *Node) UpdateInstance(tx *sql.Tx, i instance.Instance) error {
	// Don't allow updates if this instance has been assigned to a batch.
	q := `SELECT batch_id FROM instances WHERE uuid=?`
	row := tx.QueryRow(q, i.GetUUID())

	batchID := internal.INVALID_DATABASE_ID
	err := row.Scan(&batchID)
	if err != nil {
		return err
	}

	if batchID != internal.INVALID_DATABASE_ID {
		q = `SELECT name FROM batches WHERE id=?`
		row = tx.QueryRow(q, batchID)

		batchName := ""
		err := row.Scan(&batchName)
		if err != nil {
			return err
		}

		return fmt.Errorf("Cannot update instance '%s' while assigned to batch '%s'", i.GetName(), batchName)
	}

	// Update instance in the database.
	q = `UPDATE instances SET inventory_path=?,migration_status=?,migration_status_string=?,last_update_from_source=?,source_id=?,target_id=?,batch_id=?,name=?,architecture=?,os=?,os_version=?,disks=?,nics=?,number_cpus=?,memory_in_bytes=?,use_legacy_bios=?,secure_boot_enabled=?,tpm_present=?,needs_disk_import=? WHERE uuid=?`

	internalInstance, ok := i.(*instance.InternalInstance)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalInstance?")
	}

	marshalledLastUpdateFromSource, err := internalInstance.LastUpdateFromSource.MarshalText()
	if err != nil {
		return err
	}

	marshalledDisks, err := json.Marshal(internalInstance.Disks)
	if err != nil {
		return err
	}

	marshalledNICs, err := json.Marshal(internalInstance.NICs)
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, internalInstance.InventoryPath, internalInstance.MigrationStatus, internalInstance.MigrationStatusString, marshalledLastUpdateFromSource, internalInstance.SourceID, internalInstance.TargetID, internalInstance.BatchID, internalInstance.Name, internalInstance.Architecture, internalInstance.OS, internalInstance.OSVersion, marshalledDisks, marshalledNICs, internalInstance.NumberCPUs, internalInstance.MemoryInBytes, internalInstance.UseLegacyBios, internalInstance.SecureBootEnabled, internalInstance.TPMPresent, internalInstance.NeedsDiskImport, internalInstance.UUID)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Instance with UUID '%s' doesn't exist, can't update", internalInstance.UUID)
	}

	return nil
}

func (n *Node) getInstancesHelper(tx *sql.Tx, UUID uuid.UUID) ([]instance.Instance, error) {
	ret := []instance.Instance{}

	// Get all instances in the database.
	q := `SELECT uuid,inventory_path,migration_status,migration_status_string,last_update_from_source,source_id,target_id,batch_id,name,architecture,os,os_version,disks,nics,number_cpus,memory_in_bytes,use_legacy_bios,secure_boot_enabled,tpm_present,needs_disk_import FROM instances`
	var rows *sql.Rows
	var err error
	if UUID != [16]byte{} {
		q += ` WHERE uuid=?`
		rows, err = tx.Query(q, UUID)
	} else {
		q += ` ORDER BY name`
		rows, err = tx.Query(q)
	}

	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		newInstance := &instance.InternalInstance{}
		marshalledLastUpdateFromSource := ""
		marshalledDisks := ""
		marshalledNICs := ""

		err := rows.Scan(&newInstance.UUID, &newInstance.InventoryPath, &newInstance.MigrationStatus, &newInstance.MigrationStatusString, &marshalledLastUpdateFromSource, &newInstance.SourceID, &newInstance.TargetID, &newInstance.BatchID, &newInstance.Name, &newInstance.Architecture, &newInstance.OS, &newInstance.OSVersion, &marshalledDisks, &marshalledNICs, &newInstance.NumberCPUs, &newInstance.MemoryInBytes, &newInstance.UseLegacyBios, &newInstance.SecureBootEnabled, &newInstance.TPMPresent, &newInstance.NeedsDiskImport)
		if err != nil {
			return nil, err
		}

		err = newInstance.LastUpdateFromSource.UnmarshalText([]byte(marshalledLastUpdateFromSource))
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledDisks), &newInstance.Disks)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledNICs), &newInstance.NICs)
		if err != nil {
			return nil, err
		}

		ret = append(ret, newInstance)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}

func (n *Node) GetAllInstancesByState(tx *sql.Tx, status api.MigrationStatusType) ([]instance.Instance, error) {
	ret := []instance.Instance{}

	instances, err := n.GetAllInstances(tx)
	if err != nil {
		return nil, err
	}

	for _, i := range instances {
		if i.GetMigrationStatus() == status {
			ret = append(ret, i)
		}
	}

	return ret, nil
}

func (n *Node) UpdateInstanceStatus(tx *sql.Tx, UUID uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) error {
	q := `UPDATE instances SET migration_status=?,migration_status_string=?,needs_disk_import=? WHERE uuid=?`
	_, err := tx.Exec(q, status, statusString, needsDiskImport, UUID)

	return err
}
