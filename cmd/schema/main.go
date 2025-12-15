package main

import (
	"github.com/FuturFusion/migration-manager/internal/db"
)

func main() {
	err := db.SchemaDotGo()
	if err != nil {
		panic(err)
	}
}
