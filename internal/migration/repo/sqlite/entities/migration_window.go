package entities

import (
	"time"
)

// Code generation directives.
//
//generate-database:mapper target migration_window.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e migration_window objects table=migration_windows
//generate-database:mapper stmt -e migration_window objects-by-ID table=migration_windows
//generate-database:mapper stmt -e migration_window objects-by-Start-and-End-and-Lockout table=migration_windows
//generate-database:mapper stmt -e migration_window id table=migration_windows
//generate-database:mapper stmt -e migration_window create table=migration_windows
//generate-database:mapper stmt -e migration_window update table=migration_windows
//generate-database:mapper stmt -e migration_window delete-by-Start-and-End-and-Lockout table=migration_windows
//
//generate-database:mapper method -e migration_window ID table=migration_windows
//generate-database:mapper method -e migration_window GetOne table=migration_windows
//generate-database:mapper method -e migration_window GetMany table=migration_windows
//generate-database:mapper method -e migration_window Create table=migration_windows
//generate-database:mapper method -e migration_window Update table=migration_windows
//generate-database:mapper method -e migration_window DeleteOne-by-Start-and-End-and-Lockout table=migration_windows

type MigrationWindowFilter struct {
	ID      *int64
	Start   *time.Time
	End     *time.Time
	Lockout *time.Time
}
