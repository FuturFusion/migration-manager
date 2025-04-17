package properties

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func compareVersions[T api.TargetType | api.SourceType](t T, srcVer string, defVer string) error {
	switch t := any(t).(type) {
	case api.TargetType:
		return compareTargetVersions(t, srcVer, defVer)
	case api.SourceType:
		return compareSourceVersions(t, srcVer, defVer)
	}

	return fmt.Errorf("Invalid source or target: %q", t)
}

func compareSourceVersions(src api.SourceType, srcVer string, defVer string) error {
	switch src {
	case api.SOURCETYPE_VMWARE:
		// Remove extra characters to leave only the semantic version.
		srcVer, _, _ = strings.Cut(srcVer, "u")
		srcVer, _, _ = strings.Cut(srcVer, "-")

		// Major versions must match.
		srcMajor := semver.Major("v" + srcVer)
		defMajor := semver.Major("v" + defVer)
		if semver.Compare(srcMajor, defMajor) == 0 {
			return nil
		}

		// Use v8 definitions for v7.
		if srcMajor == "v7" && defMajor == "v8" {
			return nil
		}

		return fmt.Errorf("Property definition version %q does not support version %q for %q", defVer, srcVer, src)
	}

	return fmt.Errorf("Unsupported source %q", src)
}

func compareTargetVersions(tgt api.TargetType, tgtVer string, defVer string) error {
	switch tgt {
	case api.TARGETTYPE_INCUS:
		// Major versions must match.
		if semver.Compare(semver.Major("v"+tgtVer), semver.Major("v"+defVer)) == 0 {
			return nil
		}

		return fmt.Errorf("Property definition version %q does not support version %q for %q", defVer, tgtVer, tgt)
	}

	return fmt.Errorf("Unsupported target %q", tgt)
}

func validateSourceVersion(t api.SourceType, version string) error {
	switch t {
	case api.SOURCETYPE_VMWARE:
		if semver.Canonical("v"+version) == "" {
			return fmt.Errorf("Source %q version %q is not a valid semantic version", t, version)
		}

		return nil
	}

	return fmt.Errorf("Unsupported source %q", t)
}

func validateTargetVersion(t api.TargetType, version string) error {
	switch t {
	case api.TARGETTYPE_INCUS:
		if semver.Canonical("v"+version) == "" {
			return fmt.Errorf("Target %q version %q is not a valid semantic version", t, version)
		}

		return nil
	}

	return fmt.Errorf("Unsupported target %q", t)
}
