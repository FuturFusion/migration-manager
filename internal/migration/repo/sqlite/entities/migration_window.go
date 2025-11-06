package entities

import "github.com/FuturFusion/migration-manager/internal/migration"

// Code generation directives.
//
//generate-database:mapper target migration_window.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e migration_window objects table=migration_windows
//generate-database:mapper stmt -e migration_window objects-by-ID table=migration_windows
//generate-database:mapper stmt -e migration_window objects-by-Name table=migration_windows
//generate-database:mapper stmt -e migration_window objects-by-Batch table=migration_windows
//generate-database:mapper stmt -e migration_window objects-by-Name-and-Batch table=migration_windows
//generate-database:mapper stmt -e migration_window id table=migration_windows
//generate-database:mapper stmt -e migration_window create table=migration_windows
//generate-database:mapper stmt -e migration_window update table=migration_windows
//generate-database:mapper stmt -e migration_window delete-by-Name-and-Batch table=migration_windows
//
//generate-database:mapper method -e migration_window ID table=migration_windows
//generate-database:mapper method -e migration_window GetOne table=migration_windows
//generate-database:mapper method -e migration_window GetMany table=migration_windows
//generate-database:mapper method -e migration_window Create table=migration_windows
//generate-database:mapper method -e migration_window Update table=migration_windows
//generate-database:mapper method -e migration_window DeleteOne-by-Name-and-Batch table=migration_windows

type MigrationWindow = migration.Window

type MigrationWindowFilter struct {
	ID    *int64
	Name  *string
	Batch *string
}
