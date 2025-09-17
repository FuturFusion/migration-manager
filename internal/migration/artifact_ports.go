package migration

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out artifact_service_mock_gen_test.go -rm . ArtifactService

type ArtifactService interface {
	Create(ctx context.Context, artifact Artifact) (Artifact, error)
	GetAll(ctx context.Context) (Artifacts, error)
	GetAllByType(ctx context.Context, artType api.ArtifactType) (Artifacts, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Artifact, error)
	Update(ctx context.Context, id uuid.UUID, artifact *Artifact) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error

	WriteFile(id uuid.UUID, fileName string, reader io.ReadCloser) error
	DeleteFile(id uuid.UUID, fileName string) error
	GetFiles(id uuid.UUID) ([]string, error)
	FileDirectory(id uuid.UUID) string
	HasRequiredArtifactsForInstance(artifacts Artifacts, inst Instance) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/artifact_repo_mock_gen.go -rm . ArtifactRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i ArtifactRepo -t ../logger/slog.gotmpl -o ./repo/middleware/artifact_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i ArtifactRepo -t prometheus -o ./repo/middleware/artifact_prometheus_gen.go

type ArtifactRepo interface {
	Create(ctx context.Context, artifact Artifact) (int64, error)
	GetAll(ctx context.Context) (Artifacts, error)
	GetAllByType(ctx context.Context, artType api.ArtifactType) (Artifacts, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Artifact, error)
	Update(ctx context.Context, id uuid.UUID, artifact *Artifact) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error
}
