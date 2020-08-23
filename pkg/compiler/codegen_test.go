package compiler

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
)

func TestConvertToken(t *testing.T) {
	type testCase struct {
		name   string
		token  token.Token
		opcode opcode.Opcode
		typ    types.Type
	}

	testCases := []testCase{
		{"ADD (number)",
			token.ADD,
			opcode.ADD,
			types.Typ[types.Int],
		},
		{"ADD (string)",
			token.ADD,
			opcode.CAT,
			types.Typ[types.String],
		},
		{"SUB",
			token.SUB,
			opcode.SUB,
			nil,
		},
		{"MUL",
			token.MUL,
			opcode.MUL,
			nil,
		},
		{"QUO",
			token.QUO,
			opcode.DIV,
			nil,
		},
		{"REM",
			token.REM,
			opcode.MOD,
			nil,
		},
		{"ADD_ASSIGN (number)",
			token.ADD_ASSIGN,
			opcode.ADD,
			types.Typ[types.Int],
		},
		{"ADD_ASSIGN (string)",
			token.ADD_ASSIGN,
			opcode.CAT,
			types.Typ[types.String],
		},
		{"SUB_ASSIGN",
			token.SUB_ASSIGN,
			opcode.SUB,
			nil,
		},
		{"MUL_ASSIGN",
			token.MUL_ASSIGN,
			opcode.MUL,
			nil,
		},
		{"QUO_ASSIGN",
			token.QUO_ASSIGN,
			opcode.DIV,
			nil,
		},
		{"REM_ASSIGN",
			token.REM_ASSIGN,
			opcode.MOD,
			nil,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) { eval(t, tcase.token, tcase.opcode, tcase.typ) })
	}
}

func eval(t *testing.T, token token.Token, opcode opcode.Opcode, typ types.Type) {
	op, err := convertToken(token, typ)
	assert.NoError(t, err)
	assert.Equal(t, opcode, op)
}
