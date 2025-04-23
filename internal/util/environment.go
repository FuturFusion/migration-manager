package util

import (
	"os"
)

// InTestingMode returns whether migration manager is running in testing mode.
func InTestingMode() bool {
	return os.Getenv("MIGRATION_MANAGER_TESTING") != ""
}
