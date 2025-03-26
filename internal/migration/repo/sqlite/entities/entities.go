package entities

//go:generate go run github.com/lxc/incus/v6/cmd/generate-database db mapper generate -b mapper_boilerplate.go -p "github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities" -p "github.com/FuturFusion/migration-manager/internal/migration"
//go:generate gofmt -s -w .
//go:generate go run golang.org/x/tools/cmd/goimports -w .
