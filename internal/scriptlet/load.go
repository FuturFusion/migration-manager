package scriptlet

import (
	"github.com/lxc/incus/v6/shared/scriptlet"
	"go.starlark.net/starlark"
)

const batchPlacementFunc = "placement"

// BatchPlacement is the name used in Starlark for the batch placement scriptlet.
func BatchPlacement(batchName string) string {
	return batchName + "_" + batchPlacementFunc
}

// BatchPlacementValidate validates the batch placement scriptlet.
func BatchPlacementValidate(src string, batchName string) error {
	return scriptlet.Validate(BatchPlacementCompile, BatchPlacement(batchName), src, scriptlet.Declaration{
		scriptlet.Required(batchPlacementFunc): {"instance", "batch"},
	})
}

// BatchPlacementCompile compiles the batch placement scriptlet.
func BatchPlacementCompile(name string, src string) (*starlark.Program, error) {
	return scriptlet.Compile(name, src, []string{
		"log_info",
		"log_warn",
		"log_error",

		"set_target",
		"set_project",
		"set_pool",
		"set_network",
	})
}

// BatchPlacementProgram returns the precompiled batch placement scriptlet program.
func BatchPlacementProgram(loader *scriptlet.Loader, batchName string) (*starlark.Program, *starlark.Thread, error) {
	return loader.Program("Batch placement", BatchPlacement(batchName))
}

// BatchPlacementSet compiles the batch placement scriptlet into memory for use with BatchPlacementRun.
// If empty src is provided the current program is deleted.
func BatchPlacementSet(loader *scriptlet.Loader, src string, batchName string) error {
	return loader.Set(BatchPlacementCompile, BatchPlacement(batchName), src)
}
