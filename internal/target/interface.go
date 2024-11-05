package target

import (
	"context"
)

// Interface definition for all migration manager targets.
type Target interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	SetInsecureTLS(insecure bool) error
	SetClientTLSCredentials(key string, cert string) error
	IsConnected() bool

	GetName() string
	GetDatabaseID() int
	SetDatabaseID(id int)

	SetProfile(profile string) error
	SetProject(project string) error
}
