package entities

// Code generation directives.
//
//generate-database:mapper target network.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e network objects
//generate-database:mapper stmt -e network objects-by-Name
//generate-database:mapper stmt -e network names
//generate-database:mapper stmt -e network id
//generate-database:mapper stmt -e network create
//generate-database:mapper stmt -e network update
//generate-database:mapper stmt -e network rename
//generate-database:mapper stmt -e network delete-by-Name
//
//generate-database:mapper method -e network ID
//generate-database:mapper method -e network Exists
//generate-database:mapper method -e network GetOne
//generate-database:mapper method -e network GetMany
//generate-database:mapper method -e network GetNames
//generate-database:mapper method -e network Create
//generate-database:mapper method -e network Update
//generate-database:mapper method -e network Rename
//generate-database:mapper method -e network DeleteOne-by-Name

type NetworkFilter struct {
	Name *string
}
