/*
Package dbconfig is a micropackage that contains storage DB configuration options.
*/
package dbconfig

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

type (
	// DBConfiguration describes configuration for DB. Supported types:
	// [LevelDB], [BoltDB] or [InMemoryDB] (not recommended for production usage).
	DBConfiguration struct {
		Type           string         `yaml:"Type"`
		LevelDBOptions LevelDBOptions `yaml:"LevelDBOptions"`
		BoltDBOptions  BoltDBOptions  `yaml:"BoltDBOptions"`
	}
	// LevelDBOptions configuration for LevelDB.
	LevelDBOptions struct {
		DataDirectoryPath string `yaml:"DataDirectoryPath"`
		ReadOnly          bool   `yaml:"ReadOnly"`
		// WriteBufferSize defines maximum size of a 'memdb' before flushed to
		// 'sorted table'. Default is 4MiB.
		// Can be specified as a number or as an expression like "256 * 1024"
		WriteBufferSize string `yaml:"WriteBufferSize,omitempty"`
		// BlockSize is the minimum uncompressed size in bytes of each 'sorted table'
		// block. Default is 4KiB.
		// Can be specified as a number or as an expression like "32 * 1024"
		BlockSize string `yaml:"BlockSize,omitempty"`
		// BlockCacheCapacity defines the capacity of the 'sorted table' block caching.
		// Default is 8MiB.
		// Can be specified as a number or as an expression like "8 * 1024 * 1024"
		BlockCacheCapacity string `yaml:"BlockCacheCapacity,omitempty"`
		// CompactionTableSize limits size of 'sorted table' that compaction generates.
		// Default is 2MiB.
		// Can be specified as a number or as an expression like "2 * 1024 * 1024"
		CompactionTableSize string `yaml:"CompactionTableSize,omitempty"`
		// CompactionL0Trigger defines number of 'sorted table' at level-0 that will
		// trigger compaction. Default is 4.
		CompactionL0Trigger int `yaml:"CompactionL0Trigger,omitempty"`
		// OpenFilesCacheCapacity defines the capacity of the open files caching.
		// Default is 500 (200 on MacOS).
		OpenFilesCacheCapacity int `yaml:"OpenFilesCacheCapacity,omitempty"`
	}
	// BoltDBOptions configuration for BoltDB.
	BoltDBOptions struct {
		FilePath string `yaml:"FilePath"`
		ReadOnly bool   `yaml:"ReadOnly"`
	}
)

// EvaluateExpression parses and evaluates a simple mathematical expression.
// Supports basic operations: +, -, *, /, and constants like KB, MB, GB.
// Examples: "256 * 1024", "4 * 1024 * 1024", "1 + 2 * 3"
func EvaluateExpression(expr string) (int, error) {
	// If the string is empty, return 0
	if expr == "" {
		return 0, nil
	}

	// Try to parse as a simple integer first
	if val, err := strconv.Atoi(expr); err == nil {
		return val, nil
	}

	// Replace common size constants
	expr = strings.ReplaceAll(expr, "KB", "* 1024")
	expr = strings.ReplaceAll(expr, "MB", "* 1024 * 1024")
	expr = strings.ReplaceAll(expr, "GB", "* 1024 * 1024 * 1024")

	// Parse the expression
	e, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse expression '%s': %w", expr, err)
	}

	// Evaluate the expression
	return evaluateAst(e)
}

// evaluateAst recursively evaluates an AST expression
func evaluateAst(expr ast.Expr) (int, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return 0, fmt.Errorf("only integer literals are supported, got %v", e.Kind)
		}
		return strconv.Atoi(e.Value)

	case *ast.BinaryExpr:
		x, err := evaluateAst(e.X)
		if err != nil {
			return 0, err
		}
		y, err := evaluateAst(e.Y)
		if err != nil {
			return 0, err
		}

		switch e.Op {
		case token.ADD:
			return x + y, nil
		case token.SUB:
			return x - y, nil
		case token.MUL:
			return x * y, nil
		case token.QUO:
			if y == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return x / y, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %v", e.Op)
		}

	case *ast.ParenExpr:
		return evaluateAst(e.X)

	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}
