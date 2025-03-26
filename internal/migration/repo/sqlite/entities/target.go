package entities

// Code generation directives.
//
//generate-database:mapper target target.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e target objects
//generate-database:mapper stmt -e target objects-by-Name
//generate-database:mapper stmt -e target names
//generate-database:mapper stmt -e target id
//generate-database:mapper stmt -e target create
//generate-database:mapper stmt -e target update
//generate-database:mapper stmt -e target rename
//generate-database:mapper stmt -e target delete-by-Name
//
//generate-database:mapper method -e target ID
//generate-database:mapper method -e target Exists
//generate-database:mapper method -e target GetOne
//generate-database:mapper method -e target GetMany
//generate-database:mapper method -e target GetNames
//generate-database:mapper method -e target Create
//generate-database:mapper method -e target Update
//generate-database:mapper method -e target Rename
//generate-database:mapper method -e target DeleteOne-by-Name

type TargetFilter struct {
	Name *string
}
