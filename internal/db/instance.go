package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddInstance(tx *sql.Tx, i instance.Instance) error {
	internalInstance, ok := i.(*instance.InternalInstance)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalInstance?")
	}

	// Add instance to the database.
	q := `INSERT INTO instances (uuid,inventory_path,annotation,migration_status,migration_status_string,last_update_from_source,source_id,target_id,batch_id,guest_tools_version,architecture,hardware_version,os,os_version,devices,disks,nics,snapshots,cpu,memory,use_legacy_bios,secure_boot_enabled,tpm_present,needs_disk_import,secret_token) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	marshalledLastUpdateFromSource, err := internalInstance.LastUpdateFromSource.MarshalText()
	if err != nil {
		return err
	}

	marshalledDevices, err := json.Marshal(internalInstance.Devices)
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

	marshalledSnapshots, err := json.Marshal(internalInstance.Snapshots)
	if err != nil {
		return err
	}

	marshalledCPU, err := json.Marshal(internalInstance.CPU)
	if err != nil {
		return err
	}

	marshalledMemory, err := json.Marshal(internalInstance.Memory)
	if err != nil {
		return err
	}

	_, err = tx.Exec(q, internalInstance.UUID, internalInstance.InventoryPath, internalInstance.Annotation, internalInstance.MigrationStatus, internalInstance.MigrationStatusString, marshalledLastUpdateFromSource, internalInstance.SourceID, internalInstance.TargetID, internalInstance.BatchID, internalInstance.GuestToolsVersion, internalInstance.Architecture, internalInstance.HardwareVersion, internalInstance.OS, internalInstance.OSVersion, marshalledDevices, marshalledDisks, marshalledNICs, marshalledSnapshots, marshalledCPU, marshalledMemory, internalInstance.UseLegacyBios, internalInstance.SecureBootEnabled, internalInstance.TPMPresent, internalInstance.NeedsDiskImport, internalInstance.SecretToken)

	return mapDBError(err)
}

func (n *Node) GetInstance(tx *sql.Tx, UUID uuid.UUID) (instance.Instance, error) {
	ret, err := n.getInstancesHelper(tx, UUID)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No instance exists with UUID %q", UUID)
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

	if i.GetBatchID() != nil || i.IsMigrating() {
		return fmt.Errorf("Cannot delete instance %q: Either assigned to a batch or currently migrating", i.GetInventoryPath())
	}

	// Delete any corresponding override first.
	q := `DELETE FROM instance_overrides WHERE uuid=?`
	_, err = tx.Exec(q, UUID)
	if err != nil {
		return mapDBError(err)
	}

	// Delete the instance from the database.
	q = `DELETE FROM instances WHERE uuid=?`
	result, err := tx.Exec(q, UUID)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Instance with UUID %q doesn't exist, can't delete", UUID)
	}

	return nil
}

func (n *Node) UpdateInstance(tx *sql.Tx, i instance.Instance) error {
	// Don't allow updates if this instance has been assigned to a batch.
	q := `SELECT batch_id FROM instances WHERE uuid=?`
	row := tx.QueryRow(q, i.GetUUID())

	var batchID *int
	err := row.Scan(&batchID)
	if err != nil {
		return mapDBError(err)
	}

	if batchID != nil {
		q = `SELECT name FROM batches WHERE id=?`
		row = tx.QueryRow(q, *batchID)

		batchName := ""
		err := row.Scan(&batchName)
		if err != nil {
			return mapDBError(err)
		}

		return fmt.Errorf("Cannot update instance %q while assigned to batch %q", i.GetInventoryPath(), batchName)
	}

	// Update instance in the database.
	q = `UPDATE instances SET inventory_path=?,annotation=?,migration_status=?,migration_status_string=?,last_update_from_source=?,source_id=?,target_id=?,batch_id=?,guest_tools_version=?,architecture=?,hardware_version=?,os=?,os_version=?,devices=?,disks=?,nics=?,snapshots=?,cpu=?,memory=?,use_legacy_bios=?,secure_boot_enabled=?,tpm_present=?,needs_disk_import=?,secret_token=? WHERE uuid=?`

	internalInstance, ok := i.(*instance.InternalInstance)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalInstance?")
	}

	marshalledLastUpdateFromSource, err := internalInstance.LastUpdateFromSource.MarshalText()
	if err != nil {
		return err
	}

	marshalledDevices, err := json.Marshal(internalInstance.Devices)
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

	marshalledSnapshots, err := json.Marshal(internalInstance.Snapshots)
	if err != nil {
		return err
	}

	marshalledCPU, err := json.Marshal(internalInstance.CPU)
	if err != nil {
		return err
	}

	marshalledMemory, err := json.Marshal(internalInstance.Memory)
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, internalInstance.InventoryPath, internalInstance.Annotation, internalInstance.MigrationStatus, internalInstance.MigrationStatusString, marshalledLastUpdateFromSource, internalInstance.SourceID, internalInstance.TargetID, internalInstance.BatchID, internalInstance.GuestToolsVersion, internalInstance.Architecture, internalInstance.HardwareVersion, internalInstance.OS, internalInstance.OSVersion, marshalledDevices, marshalledDisks, marshalledNICs, marshalledSnapshots, marshalledCPU, marshalledMemory, internalInstance.UseLegacyBios, internalInstance.SecureBootEnabled, internalInstance.TPMPresent, internalInstance.NeedsDiskImport, internalInstance.SecretToken, internalInstance.UUID)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Instance with UUID %q doesn't exist, can't update", internalInstance.UUID)
	}

	return nil
}

func (n *Node) getInstancesHelper(tx *sql.Tx, UUID uuid.UUID) ([]instance.Instance, error) {
	ret := []instance.Instance{}

	// Get all instances in the database.
	q := `SELECT uuid,inventory_path,annotation,migration_status,migration_status_string,last_update_from_source,source_id,target_id,batch_id,guest_tools_version,architecture,hardware_version,os,os_version,devices,disks,nics,snapshots,cpu,memory,use_legacy_bios,secure_boot_enabled,tpm_present,needs_disk_import,secret_token FROM instances`
	var rows *sql.Rows
	var err error
	if UUID != [16]byte{} {
		q += ` WHERE uuid=?`
		rows, err = tx.Query(q, UUID)
	} else {
		q += ` ORDER BY inventory_path`
		rows, err = tx.Query(q)
	}

	if err != nil {
		return nil, mapDBError(err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		newInstance := &instance.InternalInstance{}
		marshalledLastUpdateFromSource := ""
		marshalledDevices := ""
		marshalledDisks := ""
		marshalledNICs := ""
		marshalledSnapshots := ""
		marshalledCPU := ""
		marshalledMemory := ""

		err := rows.Scan(&newInstance.UUID, &newInstance.InventoryPath, &newInstance.Annotation, &newInstance.MigrationStatus, &newInstance.MigrationStatusString, &marshalledLastUpdateFromSource, &newInstance.SourceID, &newInstance.TargetID, &newInstance.BatchID, &newInstance.GuestToolsVersion, &newInstance.Architecture, &newInstance.HardwareVersion, &newInstance.OS, &newInstance.OSVersion, &marshalledDevices, &marshalledDisks, &marshalledNICs, &marshalledSnapshots, &marshalledCPU, &marshalledMemory, &newInstance.UseLegacyBios, &newInstance.SecureBootEnabled, &newInstance.TPMPresent, &newInstance.NeedsDiskImport, &newInstance.SecretToken)
		if err != nil {
			return nil, err
		}

		err = newInstance.LastUpdateFromSource.UnmarshalText([]byte(marshalledLastUpdateFromSource))
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledDevices), &newInstance.Devices)
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

		err = json.Unmarshal([]byte(marshalledSnapshots), &newInstance.Snapshots)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledCPU), &newInstance.CPU)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledMemory), &newInstance.Memory)
		if err != nil {
			return nil, err
		}

		overrides, err := n.GetInstanceOverride(tx, newInstance.UUID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		newInstance.Overrides = overrides

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

	return mapDBError(err)
}
