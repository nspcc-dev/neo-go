package compiler

import (
	"go/token"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
)

func TestConvertToken(t *testing.T) {
	type testCase struct {
		name   string
		token  token.Token
		opcode opcode.Opcode
	}

	testCases := []testCase{
		{"ADD",
			token.ADD,
			opcode.ADD,
		},
		{"SUB",
			token.SUB,
			opcode.SUB,
		},
		{"MUL",
			token.MUL,
			opcode.MUL,
		},
		{"QUO",
			token.QUO,
			opcode.DIV,
		},
		{"REM",
			token.REM,
			opcode.MOD,
		},
		{"ADD_ASSIGN",
			token.ADD_ASSIGN,
			opcode.ADD,
		},
		{"SUB_ASSIGN",
			token.SUB_ASSIGN,
			opcode.SUB,
		},
		{"MUL_ASSIGN",
			token.MUL_ASSIGN,
			opcode.MUL,
		},
		{"QUO_ASSIGN",
			token.QUO_ASSIGN,
			opcode.DIV,
		},
		{"REM_ASSIGN",
			token.REM_ASSIGN,
			opcode.MOD,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) { eval(t, tcase.token, tcase.opcode) })
	}
}

func eval(t *testing.T, token token.Token, opcode opcode.Opcode) {
	codegen := &codegen{prog: newProgram()}
	codegen.convertToken(token)
	readOpcode := codegen.prog.Bytes()
	assert.Equal(t, []byte{byte(opcode)}, readOpcode)
}
