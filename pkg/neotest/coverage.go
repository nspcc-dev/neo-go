package neotest

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

var rawCoverage = make(map[util.Uint160]*scriptRawCoverage)

var enabled bool
var coverProfile = ""

type scriptRawCoverage struct {
	debugInfo      *compiler.DebugInfo
	offsetsVisited []int
}

type coverBlock struct {
	startLine uint // Line number for block start.
	startCol  uint // Column number for block start.
	endLine   uint // Line number for block end.
	endCol    uint // Column number for block end.
	stmts     uint // Number of statements included in this block.
	counts    uint
}

type documentName = string

func isCoverageEnabled() bool {
	if enabled {
		return true
	}
	const coverProfileFlag = "test.coverprofile"
	flag.VisitAll(func(f *flag.Flag) {
		if f.Name == coverProfileFlag && f.Value != nil {
			enabled = true
			coverProfile = f.Value.String()
		}
	})
	if enabled {
		// this is needed so go cover tool doesn't overwrite
		// the file with our coverage when all tests are done
		flag.Set(coverProfileFlag, "")
	}
	return enabled
}

func coverageHook() vm.OnExecHook {
	return func(scriptHash util.Uint160, offset int, opcode opcode.Opcode) {
		if cov, ok := rawCoverage[scriptHash]; ok {
			cov.offsetsVisited = append(cov.offsetsVisited, offset)
		}
	}
}

func reportCoverage() {
	f, err := os.Create(coverProfile)
	if err != nil {
		panic(fmt.Sprintf("coverage: can't create file '%s' to write coverage report", coverProfile))
	}
	defer f.Close()
	writeCoverageReport(f)
}

func writeCoverageReport(w io.Writer) {
	fmt.Fprintf(w, "mode: set\n")
	cover := processCover()
	for name, blocks := range cover {
		for _, b := range blocks {
			c := 0
			if b.counts > 0 {
				c = 1
			}
			fmt.Fprintf(w, "%s:%d.%d,%d.%d %d %d\n", name,
				b.startLine, b.startCol,
				b.endLine, b.endCol,
				b.stmts,
				c,
			)
		}
	}
}

func processCover() map[documentName][]coverBlock {
	documents := make(map[documentName]struct{})
	for _, scriptRawCoverage := range rawCoverage {
		for _, documentName := range scriptRawCoverage.debugInfo.Documents {
			documents[documentName] = struct{}{}
		}
	}

	cover := make(map[documentName][]coverBlock)

	for documentName := range documents {
		mappedBlocks := make(map[int]*coverBlock)

		for _, scriptRawCoverage := range rawCoverage {
			di := scriptRawCoverage.debugInfo
			documentSeqPoints := documentSeqPoints(di, documentName)

			for _, point := range documentSeqPoints {
				b := coverBlock{
					startLine: uint(point.StartLine),
					startCol:  uint(point.StartCol),
					endLine:   uint(point.EndLine),
					endCol:    uint(point.EndCol),
					stmts:     1 + uint(point.EndLine) - uint(point.StartLine),
					counts:    0,
				}
				mappedBlocks[point.Opcode] = &b
			}
		}

		for _, scriptRawCoverage := range rawCoverage {
			di := scriptRawCoverage.debugInfo
			documentSeqPoints := documentSeqPoints(di, documentName)

			for _, offset := range scriptRawCoverage.offsetsVisited {
				for _, point := range documentSeqPoints {
					if point.Opcode == offset {
						mappedBlocks[point.Opcode].counts++
					}
				}
			}
		}

		var blocks []coverBlock
		for _, b := range mappedBlocks {
			blocks = append(blocks, *b)
		}
		cover[documentName] = blocks
	}

	return cover
}

func documentSeqPoints(di *compiler.DebugInfo, doc documentName) []compiler.DebugSeqPoint {
	var res []compiler.DebugSeqPoint
	for _, methodDebugInfo := range di.Methods {
		for _, p := range methodDebugInfo.SeqPoints {
			if di.Documents[p.Document] == doc {
				res = append(res, p)
			}
		}
	}
	return res
}
