package migration

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/checker"
	"github.com/expr-lang/expr/conf"
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

// matchLocationAlias converts an expression to `location matches 'expression'` if the expression does not compile on its own.
func matchLocationAlias(expression string, ops ...expr.Option) string {
	config := conf.CreateNew()
	for _, op := range ops {
		op(config)
	}

	// Allow undefined variables as we don't care about the underlying object's fields for this check.
	expr.AllowUndefinedVariables()(config)

	for name := range config.Disabled {
		delete(config.Builtins, name)
	}

	config.Check()

	// If the expression parses normally, don't mangle it.
	tree, err := checker.ParseCheck(expression, config)
	if err == nil {
		// If we interpret the value as an identifier or integer, then assume it's part of a path name.
		_, ok1 := tree.Node.(*ast.IdentifierNode)
		_, ok2 := tree.Node.(*ast.IntegerNode)
		if !ok1 && !ok2 {
			return expression
		}
	}

	// Otherwise, treat it as a location match.
	return "location matches " + strconv.Quote(expression)
}
