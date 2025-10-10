package util

import (
	"fmt"
	"strings"

	"github.com/lxc/incus/v6/shared/osarch"
)

func MatchArchitecture(architectures []string, candidate string) error {
	archID, err := osarch.ArchitectureID(candidate)
	if err != nil {
		return fmt.Errorf("Architecture %q is invalid: %w", candidate, err)
	}

	ids := map[int]bool{}
	for _, a := range architectures {
		id, err := osarch.ArchitectureID(a)
		if err != nil {
			return fmt.Errorf("Cannot match against architecture %q: %w", a, err)
		}

		ids[id] = true
	}

	if ids[archID] {
		return nil
	}

	return fmt.Errorf("Architecture %q does not exist in set [%q]", candidate, strings.Join(architectures, ","))
}
