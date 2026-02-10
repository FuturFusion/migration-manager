package migration

import (
	"fmt"
	"path/filepath"

	"github.com/expr-lang/expr"
)

// Common functions for parsing paths with expr-lang.
var pathFunctions = []expr.Option{
	expr.Function("path_base", func(params ...any) (any, error) {
		if len(params) != 1 {
			return nil, fmt.Errorf("invalid number of arguments, expected 1, got: %d", len(params))
		}

		path, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid argument type, expected string, got: %T", params[0])
		}

		return filepath.Base(path), nil
	}),

	expr.Function("path_dir", func(params ...any) (any, error) {
		if len(params) != 1 {
			return nil, fmt.Errorf("invalid number of arguments, expected 1, got: %d", len(params))
		}

		path, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid argument type, expected string, got: %T", params[0])
		}

		return filepath.Dir(path), nil
	}),
}
