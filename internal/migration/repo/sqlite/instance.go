package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type instance struct {
	db repo.DBTX
}

var _ migration.InstanceRepo = &instance{}

func NewInstance(db repo.DBTX) *instance {
	return &instance{
		db: db,
	}
}

func (i instance) Create(ctx context.Context, in migration.Instance) (migration.Instance, error) {
	const sqlInsert = `
INSERT INTO instances (uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token)
VALUES(:uuid, :inventory_path, :annotation, :migration_status, :migration_status_string, :last_update_from_source, :source_id, :target_id, :batch_id, :guest_tools_version, :architecture, :hardware_version, :os, :os_version, :devices, :disks, :nics, :snapshots, :cpu, :memory, :use_legacy_bios, :secure_boot_enabled, :tpm_present, :needs_disk_import, :secret_token)
RETURNING uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token;
`

	marshalledLastUpdateFromSource, err := in.LastUpdateFromSource.MarshalText()
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledDevices, err := json.Marshal(in.Devices)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledDisks, err := json.Marshal(in.Disks)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledNICs, err := json.Marshal(in.NICs)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledSnapshots, err := json.Marshal(in.Snapshots)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledCPU, err := json.Marshal(in.CPU)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledMemory, err := json.Marshal(in.Memory)
	if err != nil {
		return migration.Instance{}, err
	}

	row := i.db.QueryRowContext(ctx, sqlInsert,
		sql.Named("uuid", in.UUID),
		sql.Named("inventory_path", in.InventoryPath),
		sql.Named("annotation", in.Annotation),
		sql.Named("migration_status", in.MigrationStatus),
		sql.Named("migration_status_string", in.MigrationStatusString),
		sql.Named("last_update_from_source", marshalledLastUpdateFromSource),
		sql.Named("source_id", in.SourceID),
		sql.Named("target_id", in.TargetID),
		sql.Named("batch_id", in.BatchID),
		sql.Named("guest_tools_version", in.GuestToolsVersion),
		sql.Named("architecture", in.Architecture),
		sql.Named("hardware_version", in.HardwareVersion),
		sql.Named("os", in.OS),
		sql.Named("os_version", in.OSVersion),
		sql.Named("devices", marshalledDevices),
		sql.Named("disks", marshalledDisks),
		sql.Named("nics", marshalledNICs),
		sql.Named("snapshots", marshalledSnapshots),
		sql.Named("cpu", marshalledCPU),
		sql.Named("memory", marshalledMemory),
		sql.Named("use_legacy_bios", in.UseLegacyBios),
		sql.Named("secure_boot_enabled", in.SecureBootEnabled),
		sql.Named("tpm_present", in.TPMPresent),
		sql.Named("needs_disk_import", in.NeedsDiskImport),
		sql.Named("secret_token", in.SecretToken),
	)
	if row.Err() != nil {
		return migration.Instance{}, row.Err()
	}

	return scanInstance(row)
}

func (i instance) GetAll(ctx context.Context) (migration.Instances, error) {
	const sqlGetAll = `
SELECT uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token
FROM instances
ORDER BY inventory_path;
`

	rows, err := i.db.QueryContext(ctx, sqlGetAll)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var instances migration.Instances
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}

		instances = append(instances, instance)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return instances, nil
}

func (i instance) GetAllByBatchID(ctx context.Context, batchID int) (migration.Instances, error) {
	const sqlGetAllByState = `
SELECT uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token
FROM instances
WHERE batch_id=:batch_id
ORDER BY inventory_path;
`

	rows, err := i.db.QueryContext(ctx, sqlGetAllByState,
		sql.Named("batch_id", batchID),
	)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var instances migration.Instances
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}

		instances = append(instances, instance)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return instances, nil
}

func (i instance) GetAllByState(ctx context.Context, status api.MigrationStatusType) (migration.Instances, error) {
	const sqlGetAllByState = `
SELECT uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token
FROM instances
WHERE migration_status=:migration_status
ORDER BY inventory_path;
`

	rows, err := i.db.QueryContext(ctx, sqlGetAllByState,
		sql.Named("migration_status", status),
	)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var instances migration.Instances
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}

		instances = append(instances, instance)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return instances, nil
}

func (i instance) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	const sqlGetAllUUIDs = `SELECT uuid FROM instances`

	rows, err := i.db.QueryContext(ctx, sqlGetAllUUIDs)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var instanceUUIDs []uuid.UUID
	for rows.Next() {
		var instanceUUID uuid.UUID
		err := rows.Scan(&instanceUUID)
		if err != nil {
			return nil, err
		}

		instanceUUIDs = append(instanceUUIDs, instanceUUID)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return instanceUUIDs, nil
}

func (i instance) GetAllUnassigned(ctx context.Context) (migration.Instances, error) {
	const sqlGetAll = `
SELECT uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token
FROM instances
WHERE batch_id IS NULL
ORDER BY inventory_path;
`

	rows, err := i.db.QueryContext(ctx, sqlGetAll)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var instances migration.Instances
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}

		instances = append(instances, instance)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return instances, nil
}

func (i instance) GetByID(ctx context.Context, id uuid.UUID) (migration.Instance, error) {
	const sqlGetByUUID = `SELECT uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token FROM instances WHERE uuid=:uuid;`

	row := i.db.QueryRowContext(ctx, sqlGetByUUID, sql.Named("uuid", id))
	if row.Err() != nil {
		return migration.Instance{}, row.Err()
	}

	return scanInstance(row)
}

func (i instance) UpdateByID(ctx context.Context, in migration.Instance) (migration.Instance, error) {
	const sqlUpdate = `
UPDATE instances
SET inventory_path=:inventory_path, annotation=:annotation, migration_status=:migration_status, migration_status_string=:migration_status_string, last_update_from_source=:last_update_from_source, source_id=:source_id, target_id=:target_id, batch_id=:batch_id, guest_tools_version=:guest_tools_version, architecture=:architecture, hardware_version=:hardware_version, os=:os, os_version=:os_version, devices=:devices, disks=:disks, nics=:nics, snapshots=:snapshots, cpu=:cpu, memory=:memory, use_legacy_bios=:use_legacy_bios, secure_boot_enabled=:secure_boot_enabled, tpm_present=:tpm_present, needs_disk_import=:needs_disk_import, secret_token=:secret_token
WHERE uuid=:uuid
RETURNING uuid, inventory_path, annotation, migration_status, migration_status_string, last_update_from_source, source_id, target_id, batch_id, guest_tools_version, architecture, hardware_version, os, os_version, devices, disks, nics, snapshots, cpu, memory, use_legacy_bios, secure_boot_enabled, tpm_present, needs_disk_import, secret_token;
`

	marshalledLastUpdateFromSource, err := in.LastUpdateFromSource.MarshalText()
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledDevices, err := json.Marshal(in.Devices)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledDisks, err := json.Marshal(in.Disks)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledNICs, err := json.Marshal(in.NICs)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledSnapshots, err := json.Marshal(in.Snapshots)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledCPU, err := json.Marshal(in.CPU)
	if err != nil {
		return migration.Instance{}, err
	}

	marshalledMemory, err := json.Marshal(in.Memory)
	if err != nil {
		return migration.Instance{}, err
	}

	row := i.db.QueryRowContext(ctx, sqlUpdate,
		sql.Named("uuid", in.UUID),
		sql.Named("inventory_path", in.InventoryPath),
		sql.Named("annotation", in.Annotation),
		sql.Named("migration_status", in.MigrationStatus),
		sql.Named("migration_status_string", in.MigrationStatusString),
		sql.Named("last_update_from_source", marshalledLastUpdateFromSource),
		sql.Named("source_id", in.SourceID),
		sql.Named("target_id", in.TargetID),
		sql.Named("batch_id", in.BatchID),
		sql.Named("guest_tools_version", in.GuestToolsVersion),
		sql.Named("architecture", in.Architecture),
		sql.Named("hardware_version", in.HardwareVersion),
		sql.Named("os", in.OS),
		sql.Named("os_version", in.OSVersion),
		sql.Named("devices", marshalledDevices),
		sql.Named("disks", marshalledDisks),
		sql.Named("nics", marshalledNICs),
		sql.Named("snapshots", marshalledSnapshots),
		sql.Named("cpu", marshalledCPU),
		sql.Named("memory", marshalledMemory),
		sql.Named("use_legacy_bios", in.UseLegacyBios),
		sql.Named("secure_boot_enabled", in.SecureBootEnabled),
		sql.Named("tpm_present", in.TPMPresent),
		sql.Named("needs_disk_import", in.NeedsDiskImport),
		sql.Named("secret_token", in.SecretToken),
	)
	if row.Err() != nil {
		return migration.Instance{}, row.Err()
	}

	return scanInstance(row)
}

func scanInstance(row interface{ Scan(dest ...any) error }) (migration.Instance, error) {
	var instance migration.Instance
	var marshalledLastUpdateFromSource string
	var marshalledDevices string
	var marshalledDisks string
	var marshalledNICs string
	var marshalledSnapshots string
	var marshalledCPU string
	var marshalledMemory string

	err := row.Scan(
		&instance.UUID,
		&instance.InventoryPath,
		&instance.Annotation,
		&instance.MigrationStatus,
		&instance.MigrationStatusString,
		&marshalledLastUpdateFromSource,
		&instance.SourceID,
		&instance.TargetID,
		&instance.BatchID,
		&instance.GuestToolsVersion,
		&instance.Architecture,
		&instance.HardwareVersion,
		&instance.OS,
		&instance.OSVersion,
		&marshalledDevices,
		&marshalledDisks,
		&marshalledNICs,
		&marshalledSnapshots,
		&marshalledCPU,
		&marshalledMemory,
		&instance.UseLegacyBios,
		&instance.SecureBootEnabled,
		&instance.TPMPresent,
		&instance.NeedsDiskImport,
		&instance.SecretToken,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return migration.Instance{}, migration.ErrNotFound
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code == sqlite3.ErrConstraint {
				return migration.Instance{}, migration.ErrConstraintViolation
			}
		}

		return migration.Instance{}, err
	}

	err = instance.LastUpdateFromSource.UnmarshalText([]byte(marshalledLastUpdateFromSource))
	if err != nil {
		return migration.Instance{}, err
	}

	err = json.Unmarshal([]byte(marshalledDevices), &instance.Devices)
	if err != nil {
		return migration.Instance{}, err
	}

	err = json.Unmarshal([]byte(marshalledDisks), &instance.Disks)
	if err != nil {
		return migration.Instance{}, err
	}

	err = json.Unmarshal([]byte(marshalledNICs), &instance.NICs)
	if err != nil {
		return migration.Instance{}, err
	}

	err = json.Unmarshal([]byte(marshalledSnapshots), &instance.Snapshots)
	if err != nil {
		return migration.Instance{}, err
	}

	err = json.Unmarshal([]byte(marshalledCPU), &instance.CPU)
	if err != nil {
		return migration.Instance{}, err
	}

	err = json.Unmarshal([]byte(marshalledMemory), &instance.Memory)
	if err != nil {
		return migration.Instance{}, err
	}

	return instance, nil
}

func (i instance) DeleteByID(ctx context.Context, id uuid.UUID) error {
	const sqlDelete = `DELETE FROM instances WHERE uuid=:uuid;`

	result, err := i.db.ExecContext(ctx, sqlDelete, sql.Named("uuid", id))
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return migration.ErrNotFound
	}

	return nil
}

func (i instance) UpdateStatusByID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (migration.Instance, error) {
	const sqlUpdate = `
UPDATE instances
SET migration_status=:migration_status, migration_status_string=:migration_status_string, needs_disk_import=:needs_disk_import
WHERE uuid=:uuid
`

	row := i.db.QueryRowContext(ctx, sqlUpdate,
		sql.Named("uuid", id),
		sql.Named("migration_status", status),
		sql.Named("migration_status_string", statusString),
		sql.Named("needs_disk_import", needsDiskImport),
	)
	if row.Err() != nil {
		return migration.Instance{}, row.Err()
	}

	return scanInstance(row)
}

func (i instance) CreateOverrides(ctx context.Context, overrides migration.Overrides) (migration.Overrides, error) {
	const sqlInsertOverrides = `
INSERT INTO instance_overrides (uuid, last_update, comment, number_cpus, memory_in_bytes, disable_migration)
VALUES (:uuid, :last_update, :comment, :number_cpus, :memory_in_bytes, :disable_migration)
RETURNING uuid, last_update, comment, number_cpus, memory_in_bytes, disable_migration;
`

	marshalledLastUpdate, err := overrides.LastUpdate.MarshalText()
	if err != nil {
		return migration.Overrides{}, err
	}

	row := i.db.QueryRowContext(ctx, sqlInsertOverrides,
		sql.Named("uuid", overrides.UUID),
		sql.Named("last_update", marshalledLastUpdate),
		sql.Named("comment", overrides.Comment),
		sql.Named("number_cpus", overrides.NumberCPUs),
		sql.Named("memory_in_bytes", overrides.MemoryInBytes),
		sql.Named("disable_migration", overrides.DisableMigration),
	)
	if row.Err() != nil {
		return migration.Overrides{}, row.Err()
	}

	return scanInstanceOverrides(row)
}

func (i instance) GetOverridesByID(ctx context.Context, id uuid.UUID) (migration.Overrides, error) {
	const sqlGetOverridesByUUID = `
SELECT uuid, last_update, comment, number_cpus, memory_in_bytes, disable_migration
FROM instance_overrides
WHERE uuid=:uuid;
`

	row := i.db.QueryRowContext(ctx, sqlGetOverridesByUUID, sql.Named("uuid", id))
	if row.Err() != nil {
		return migration.Overrides{}, row.Err()
	}

	return scanInstanceOverrides(row)
}

func (i instance) DeleteOverridesByID(ctx context.Context, id uuid.UUID) error {
	const sqlDeleteOverrides = `DELETE FROM instance_overrides WHERE uuid=:uuid;`

	result, err := i.db.ExecContext(ctx, sqlDeleteOverrides, sql.Named("uuid", id))
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return migration.ErrNotFound
	}

	return nil
}

func (i instance) UpdateOverridesByID(ctx context.Context, overrides migration.Overrides) (migration.Overrides, error) {
	const sqlUpdateOverrides = `
UPDATE instance_overrides
SET last_update=:last_update, comment=:comment, number_cpus=:number_cpus, memory_in_bytes=:memory_in_bytes, disable_migration=:disable_migration
WHERE uuid=:uuid
RETURNING uuid, last_update, comment, number_cpus, memory_in_bytes, disable_migration;
`

	marshalledLastUpdate, err := overrides.LastUpdate.MarshalText()
	if err != nil {
		return migration.Overrides{}, err
	}

	row := i.db.QueryRowContext(ctx, sqlUpdateOverrides,
		sql.Named("uuid", overrides.UUID),
		sql.Named("last_update", marshalledLastUpdate),
		sql.Named("comment", overrides.Comment),
		sql.Named("number_cpus", overrides.NumberCPUs),
		sql.Named("memory_in_bytes", overrides.MemoryInBytes),
		sql.Named("disable_migration", overrides.DisableMigration),
	)
	if row.Err() != nil {
		return migration.Overrides{}, row.Err()
	}

	return scanInstanceOverrides(row)
}

func scanInstanceOverrides(row interface{ Scan(dest ...any) error }) (migration.Overrides, error) {
	var overrides migration.Overrides
	var marshalledLastUpdate string

	err := row.Scan(
		&overrides.UUID,
		&marshalledLastUpdate,
		&overrides.Comment,
		&overrides.NumberCPUs,
		&overrides.MemoryInBytes,
		&overrides.DisableMigration,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return migration.Overrides{}, migration.ErrNotFound
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code == sqlite3.ErrConstraint {
				return migration.Overrides{}, migration.ErrConstraintViolation
			}
		}

		return migration.Overrides{}, err
	}

	err = overrides.LastUpdate.UnmarshalText([]byte(marshalledLastUpdate))
	if err != nil {
		return migration.Overrides{}, err
	}

	return overrides, nil
}
