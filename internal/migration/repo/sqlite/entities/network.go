package entities

// Code generation directives.
//
//generate-database:mapper target network.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e network objects
//generate-database:mapper stmt -e network objects-by-SourceSpecificID
//generate-database:mapper stmt -e network objects-by-SourceSpecificID-and-Source
//generate-database:mapper stmt -e network objects-by-Source
//generate-database:mapper stmt -e network id
//generate-database:mapper stmt -e network create
//generate-database:mapper stmt -e network update
//generate-database:mapper stmt -e network delete-by-SourceSpecificID-and-Source
//
//generate-database:mapper method -e network ID
//generate-database:mapper method -e network Exists
//generate-database:mapper method -e network GetOne
//generate-database:mapper method -e network GetMany
//generate-database:mapper method -e network Create
//generate-database:mapper method -e network Update
//generate-database:mapper method -e network DeleteOne-by-SourceSpecificID-and-Source

type NetworkFilter struct {
	SourceSpecificID *string
	Source           *string
}
