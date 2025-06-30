package entities

// Code generation directives.
//
//generate-database:mapper target network.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e network objects
//generate-database:mapper stmt -e network objects-by-Identifier
//generate-database:mapper stmt -e network objects-by-Identifier-and-Source
//generate-database:mapper stmt -e network objects-by-Source
//generate-database:mapper stmt -e network id
//generate-database:mapper stmt -e network create
//generate-database:mapper stmt -e network update
//generate-database:mapper stmt -e network delete-by-Identifier-and-Source
//
//generate-database:mapper method -e network ID
//generate-database:mapper method -e network Exists
//generate-database:mapper method -e network GetOne
//generate-database:mapper method -e network GetMany
//generate-database:mapper method -e network Create
//generate-database:mapper method -e network Update
//generate-database:mapper method -e network DeleteOne-by-Identifier-and-Source

type NetworkFilter struct {
	Identifier *string
	Source     *string
}
