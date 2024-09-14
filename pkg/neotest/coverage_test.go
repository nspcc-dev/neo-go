package neotest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func resetCoverage() {
	rawCoverage = make(map[util.Uint160]*scriptRawCoverage)
}

func TestProcessCover_OneMethodOneDocument(t *testing.T) {
	t.Cleanup(resetCoverage)

	scriptHash := util.Uint160{1}
	doc := "foobar.go"
	mdi := compiler.MethodDebugInfo{
		SeqPoints: []compiler.DebugSeqPoint{
			{Opcode: 0, Document: 0, StartLine: 0, EndLine: 0},
			{Opcode: 1, Document: 0, StartLine: 1, EndLine: 1},
			{Opcode: 2, Document: 0, StartLine: 2, EndLine: 2},
		},
	}
	di := &compiler.DebugInfo{
		Documents: []string{doc},
		Methods:   []compiler.MethodDebugInfo{mdi},
	}
	contract := &Contract{Hash: scriptHash, DebugInfo: di}

	addScriptToCoverage(contract)
	coverageHook(scriptHash, 1, opcode.NOP)
	coverageHook(scriptHash, 2, opcode.NOP)
	coverageHook(scriptHash, 2, opcode.NOP)
	cover := processCover()

	require.Contains(t, cover, doc)
	documentCover := cover[doc]
	require.Equal(t, 3, len(documentCover))
	require.Contains(t, documentCover, coverBlock{startLine: 0, endLine: 0, stmts: 1, counts: 0})
	require.Contains(t, documentCover, coverBlock{startLine: 1, endLine: 1, stmts: 1, counts: 1})
	require.Contains(t, documentCover, coverBlock{startLine: 2, endLine: 2, stmts: 1, counts: 2})
}

func TestProcessCover_TwoMethodsTwoDocuments(t *testing.T) {
	t.Cleanup(resetCoverage)

	scriptHash1 := util.Uint160{1}
	scriptHash2 := util.Uint160{2}
	doc1 := "contract1.go"
	doc2 := "contract2.go"
	mdi1 := compiler.MethodDebugInfo{
		SeqPoints: []compiler.DebugSeqPoint{
			{Opcode: 0, Document: 0, StartLine: 0, EndLine: 0},
			{Opcode: 1, Document: 0, StartLine: 1, EndLine: 1},
			{Opcode: 2, Document: 0, StartLine: 2, EndLine: 2},
		},
	}
	mdi2 := compiler.MethodDebugInfo{
		SeqPoints: []compiler.DebugSeqPoint{
			{Opcode: 0, Document: 1, StartLine: 0, EndLine: 0},
			{Opcode: 1, Document: 1, StartLine: 1, EndLine: 1},
			{Opcode: 2, Document: 1, StartLine: 2, EndLine: 2},
		},
	}
	di1 := &compiler.DebugInfo{
		Documents: []string{doc1, doc2},
		Methods:   []compiler.MethodDebugInfo{mdi1},
	}
	di2 := &compiler.DebugInfo{
		Documents: []string{doc1, doc2},
		Methods:   []compiler.MethodDebugInfo{mdi2},
	}

	addScriptToCoverage(&Contract{Hash: scriptHash1, DebugInfo: di1})
	addScriptToCoverage(&Contract{Hash: scriptHash2, DebugInfo: di2})
	coverageHook(scriptHash1, 1, opcode.NOP)
	coverageHook(scriptHash1, 2, opcode.NOP)
	coverageHook(scriptHash1, 2, opcode.NOP)
	coverageHook(scriptHash2, 0, opcode.NOP)
	coverageHook(scriptHash2, 1, opcode.NOP)
	coverageHook(scriptHash2, 2, opcode.NOP)
	cover := processCover()

	require.Contains(t, cover, doc1)
	require.Contains(t, cover, doc2)
	documentCover1 := cover[doc1]
	require.Equal(t, 3, len(documentCover1))
	require.Contains(t, documentCover1, coverBlock{startLine: 0, endLine: 0, stmts: 1, counts: 0})
	require.Contains(t, documentCover1, coverBlock{startLine: 1, endLine: 1, stmts: 1, counts: 1})
	require.Contains(t, documentCover1, coverBlock{startLine: 2, endLine: 2, stmts: 1, counts: 2})
	documentCover2 := cover[doc2]
	require.Equal(t, 3, len(documentCover2))
	require.Contains(t, documentCover2, coverBlock{startLine: 0, endLine: 0, stmts: 1, counts: 1})
	require.Contains(t, documentCover2, coverBlock{startLine: 1, endLine: 1, stmts: 1, counts: 1})
	require.Contains(t, documentCover2, coverBlock{startLine: 2, endLine: 2, stmts: 1, counts: 1})
}

func TestProcessCover_BlockOverlap(t *testing.T) {
	testCases := []struct {
		name string
		mdi  compiler.MethodDebugInfo
	}{
		{
			name: "Different lines",
			mdi: compiler.MethodDebugInfo{
				SeqPoints: []compiler.DebugSeqPoint{
					{Opcode: 0, Document: 0, StartLine: 2, EndLine: 9},
					{Opcode: 1, Document: 0, StartLine: 1, EndLine: 10},
				},
			},
		},
		{
			name: "Different start lines",
			mdi: compiler.MethodDebugInfo{
				SeqPoints: []compiler.DebugSeqPoint{
					{Opcode: 0, Document: 0, StartLine: 2, EndLine: 9},
					{Opcode: 1, Document: 0, StartLine: 1, EndLine: 9},
				},
			},
		},
		{
			name: "Different end lines",
			mdi: compiler.MethodDebugInfo{
				SeqPoints: []compiler.DebugSeqPoint{
					{Opcode: 0, Document: 0, StartLine: 2, EndLine: 9},
					{Opcode: 1, Document: 0, StartLine: 2, EndLine: 10},
				},
			},
		},
		{
			name: "Different columns",
			mdi: compiler.MethodDebugInfo{
				SeqPoints: []compiler.DebugSeqPoint{
					{Opcode: 0, Document: 0, StartCol: 2, EndCol: 9},
					{Opcode: 1, Document: 0, StartCol: 1, EndCol: 10},
				},
			},
		},
		{
			name: "Different start columns",
			mdi: compiler.MethodDebugInfo{
				SeqPoints: []compiler.DebugSeqPoint{
					{Opcode: 0, Document: 0, StartCol: 2, EndCol: 9},
					{Opcode: 1, Document: 0, StartCol: 1, EndCol: 9},
				},
			},
		},
		{
			name: "Different end columns",
			mdi: compiler.MethodDebugInfo{
				SeqPoints: []compiler.DebugSeqPoint{
					{Opcode: 0, Document: 0, StartCol: 2, EndCol: 9},
					{Opcode: 1, Document: 0, StartCol: 2, EndCol: 10},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(resetCoverage)

			scriptHash := util.Uint160{1}
			doc := "foobar.go"
			di := &compiler.DebugInfo{
				Documents: []string{doc},
				Methods:   []compiler.MethodDebugInfo{tc.mdi},
			}
			contract := &Contract{Hash: scriptHash, DebugInfo: di}

			addScriptToCoverage(contract)
			cover := processCover()

			require.Contains(t, cover, doc)
			documentCover := cover[doc]
			require.Equal(t, 1, len(documentCover))
			expectedBlock := tc.mdi.SeqPoints[0]
			actualBlock := documentCover[0]
			require.Equal(t, actualBlock.startLine, uint(expectedBlock.StartLine))
			require.Equal(t, actualBlock.endLine, uint(expectedBlock.EndLine))
			require.Equal(t, actualBlock.startCol, uint(expectedBlock.StartCol))
			require.Equal(t, actualBlock.endCol, uint(expectedBlock.EndCol))
		})
	}

	t.Run("Nested", func(t *testing.T) {
		t.Cleanup(resetCoverage)

		scriptHash := util.Uint160{1}
		doc := "foobar.go"
		mdi := compiler.MethodDebugInfo{
			SeqPoints: []compiler.DebugSeqPoint{
				{Opcode: 0, Document: 0, StartLine: 0, EndLine: 4},
				{Opcode: 1, Document: 0, StartLine: 1, EndLine: 3},
				{Opcode: 2, Document: 0, StartLine: 2, EndLine: 2},
			},
		}
		di := &compiler.DebugInfo{
			Documents: []string{doc},
			Methods:   []compiler.MethodDebugInfo{mdi},
		}
		contract := &Contract{Hash: scriptHash, DebugInfo: di}

		addScriptToCoverage(contract)
		cover := processCover()

		require.Contains(t, cover, doc)
		documentCover := cover[doc]
		require.Equal(t, 1, len(documentCover))
		require.Contains(t, documentCover, coverBlock{startLine: 2, endLine: 2, stmts: 1})
	})

	t.Run("Complex", func(t *testing.T) {
		t.Cleanup(resetCoverage)

		scriptHash := util.Uint160{1}
		doc := "foobar.go"
		mdi := compiler.MethodDebugInfo{
			SeqPoints: []compiler.DebugSeqPoint{
				{Opcode: 0, Document: 0, StartLine: 0, EndLine: 0},
				{Opcode: 1, Document: 0, StartLine: 1, EndLine: 1},
				{Opcode: 2, Document: 0, StartLine: 2, EndLine: 2},
				{Opcode: 3, Document: 0, StartLine: 3, EndLine: 3},
				{Opcode: 4, Document: 0, StartLine: 4, EndLine: 4},
				{Opcode: 5, Document: 0, StartLine: 5, EndLine: 5}, // pick smaller (more specific).
				{Opcode: 6, Document: 0, StartLine: 5, EndLine: 6}, // overlap.
				{Opcode: 7, Document: 0, StartLine: 7, EndLine: 7},
				{Opcode: 8, Document: 0, StartLine: 8, EndLine: 8},
				{Opcode: 9, Document: 0, StartLine: 9, EndLine: 9},
			},
		}
		di := &compiler.DebugInfo{
			Documents: []string{doc},
			Methods:   []compiler.MethodDebugInfo{mdi},
		}
		contract := &Contract{Hash: scriptHash, DebugInfo: di}

		addScriptToCoverage(contract)
		cover := processCover()

		require.Contains(t, cover, doc)
		documentCover := cover[doc]
		require.Equal(t, 9, len(documentCover))
		require.Contains(t, documentCover, coverBlock{startLine: 5, endLine: 5, stmts: 1})
		require.NotContains(t, documentCover, coverBlock{startLine: 5, endLine: 6, stmts: 2})
	})

	t.Run("No overlap on same line", func(t *testing.T) {
		t.Cleanup(resetCoverage)

		scriptHash := util.Uint160{1}
		doc := "foobar.go"
		mdi := compiler.MethodDebugInfo{
			SeqPoints: []compiler.DebugSeqPoint{
				{Opcode: 0, Document: 0, StartLine: 0, EndLine: 0, StartCol: 1, EndCol: 2},
				{Opcode: 1, Document: 0, StartLine: 0, EndLine: 0, StartCol: 3, EndCol: 4},
			},
		}
		di := &compiler.DebugInfo{
			Documents: []string{doc},
			Methods:   []compiler.MethodDebugInfo{mdi},
		}
		contract := &Contract{Hash: scriptHash, DebugInfo: di}

		addScriptToCoverage(contract)
		cover := processCover()

		require.Contains(t, cover, doc)
		documentCover := cover[doc]
		require.Equal(t, 2, len(documentCover))
	})

	t.Run("Same intervals", func(t *testing.T) {
		t.Cleanup(resetCoverage)

		scriptHash := util.Uint160{1}
		doc := "foobar.go"
		mdi := compiler.MethodDebugInfo{
			SeqPoints: []compiler.DebugSeqPoint{
				{Opcode: 0, Document: 0},
				{Opcode: 1, Document: 0},
			},
		}
		di := &compiler.DebugInfo{
			Documents: []string{doc},
			Methods:   []compiler.MethodDebugInfo{mdi},
		}
		contract := &Contract{Hash: scriptHash, DebugInfo: di}

		addScriptToCoverage(contract)
		cover := processCover()

		require.Contains(t, cover, doc)
		documentCover := cover[doc]
		require.Equal(t, 1, len(documentCover))
	})
}
