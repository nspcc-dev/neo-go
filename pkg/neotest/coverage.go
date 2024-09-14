package neotest

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

const (
	// goCoverProfileFlag specifies the name of `go test` flag that tells it where to save coverage data.
	// Neotest coverage can be enabled only when this flag is set.
	goCoverProfileFlag = "test.coverprofile"
	// disableNeotestCover is name of the environmental variable used to explicitly disable neotest coverage.
	disableNeotestCover = "DISABLE_NEOTEST_COVER"
)

var (
	// coverageLock protects all vars below from concurrent modification when tests are run in parallel.
	coverageLock sync.Mutex
	// rawCoverage maps script hash to coverage data, collected during testing.
	rawCoverage = make(map[util.Uint160]*scriptRawCoverage)
	// flagChecked is true if `go test` coverage flag was checked at any point.
	flagChecked bool
	// coverageEnabled is true if coverage is being collected on test execution.
	coverageEnabled bool
	// coverProfile specifies the file all coverage data is written to, unless empty.
	coverProfile = ""
)

type scriptRawCoverage struct {
	debugInfo      *compiler.DebugInfo
	offsetsVisited []int
}

type coverBlock struct {
	// Line number for block start.
	startLine uint
	// Column number for block start.
	startCol uint
	// Line number for block end.
	endLine uint
	// Column number for block end.
	endCol uint
	// Number of statements included in this block.
	stmts uint
	// Number of times this block was executed.
	counts uint
}

// documentName makes it clear when a `string` maps path to the script file.
type documentName = string

type interval struct {
	compiler.DebugSeqPoint
	remove bool
}

func isCoverageEnabled() bool {
	coverageLock.Lock()
	defer coverageLock.Unlock()

	if flagChecked {
		return coverageEnabled
	}
	defer func() { flagChecked = true }()

	var disabledByEnvironment bool
	if v, ok := os.LookupEnv(disableNeotestCover); ok {
		disabled, err := strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Sprintf("coverage: error when parsing environment variable '%s', expected bool, but got '%s'", disableNeotestCover, v))
		}
		disabledByEnvironment = disabled
	}

	var goToolCoverageEnabled bool
	flag.VisitAll(func(f *flag.Flag) {
		if f.Name == goCoverProfileFlag && f.Value != nil && f.Value.String() != "" {
			goToolCoverageEnabled = true
			coverProfile = f.Value.String()
		}
	})

	coverageEnabled = !disabledByEnvironment && goToolCoverageEnabled

	if coverageEnabled {
		// This is needed so go cover tool doesn't overwrite
		// the file with our coverage when all tests are done.
		err := flag.Set(goCoverProfileFlag, "")
		if err != nil {
			panic(err)
		}
	}

	return coverageEnabled
}

var coverageHook vm.OnExecHook = func(scriptHash util.Uint160, offset int, opcode opcode.Opcode) {
	coverageLock.Lock()
	defer coverageLock.Unlock()
	if cov, ok := rawCoverage[scriptHash]; ok {
		cov.offsetsVisited = append(cov.offsetsVisited, offset)
	}
}

func reportCoverage(t testing.TB) {
	coverageLock.Lock()
	defer coverageLock.Unlock()
	f, err := os.Create(coverProfile)
	if err != nil {
		t.Fatalf("coverage: can't create file '%s' to write coverage report", coverProfile)
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

		var allDocumentSeqPoints []compiler.DebugSeqPoint
		for _, scriptRawCoverage := range rawCoverage {
			documentSeqPoints := documentSeqPoints(scriptRawCoverage.debugInfo, documentName)
			allDocumentSeqPoints = append(allDocumentSeqPoints, documentSeqPoints...)
		}
		allDocumentSeqPoints = resolveOverlaps(allDocumentSeqPoints)

		for _, point := range allDocumentSeqPoints {
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

		for _, scriptRawCoverage := range rawCoverage {
			documentSeqPoints := documentSeqPoints(scriptRawCoverage.debugInfo, documentName)
			for _, offset := range scriptRawCoverage.offsetsVisited {
				for _, point := range documentSeqPoints {
					if point.Opcode == offset {
						if _, ok := mappedBlocks[offset]; ok {
							mappedBlocks[offset].counts++
						}
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

// resolveOverlaps removes overlaps from debug points.
// Its assumed that intervals can never overlap partially.
func resolveOverlaps(points []compiler.DebugSeqPoint) []compiler.DebugSeqPoint {
	var intervals []interval
	for _, p := range points {
		intervals = append(intervals, interval{DebugSeqPoint: p})
	}
	for i := range intervals {
		for j := range intervals {
			inner := &intervals[i]
			outer := &intervals[j]
			// If interval 'i' is already removed than there exists an even smaller interval that is also included by 'j'.
			// This also ensures that if there are 2 equal intervals then at least 1 will remain.
			if i == j || inner.remove {
				continue
			}
			// Outer interval start can't be after inner interval start.
			if !(outer.StartLine < inner.StartLine || outer.StartLine == inner.StartLine && outer.StartCol <= inner.StartCol) {
				continue
			}
			// Outer interval end can't be before inner interval end.
			if !(outer.EndLine > inner.EndLine || outer.EndLine == inner.EndLine && outer.EndCol >= inner.EndCol) {
				continue
			}
			outer.remove = true
		}
	}
	var res []compiler.DebugSeqPoint
	for i, v := range intervals {
		if !v.remove {
			res = append(res, points[i])
		}
	}
	return res
}

func addScriptToCoverage(c *Contract) {
	coverageLock.Lock()
	defer coverageLock.Unlock()
	if _, ok := rawCoverage[c.Hash]; !ok {
		rawCoverage[c.Hash] = &scriptRawCoverage{debugInfo: c.DebugInfo}
	}
}
