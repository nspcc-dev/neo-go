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
	// goCoverProfileFlag specifies the name of `go test` command flag `coverprofile`
	// that tells it where to save coverage data. Neotest coverage can be enabled
	// only when this flag is set.
	goCoverProfileFlag = "test.coverprofile"
	// goCoverModeFlag specifies the name of `go test` command flag `covermode` that
	// specifies the coverage calculation mode.
	goCoverModeFlag = "test.covermode"
	// disableNeotestCover is name of the environmental variable used to explicitly disable neotest coverage.
	disableNeotestCover = "DISABLE_NEOTEST_COVER"
)

const (
	// goCoverModeSet is the name of "set" go test coverage mode.
	goCoverModeSet = "set"
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
	// coverMode is the mode of go coverage collection.
	coverMode = goCoverModeSet
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
		if f.Name == goCoverModeFlag && f.Value != nil && f.Value.String() != "" {
			coverMode = f.Value.String()
		}
	})

	coverageEnabled = !disabledByEnvironment && goToolCoverageEnabled

	if coverageEnabled {
		if coverMode != goCoverModeSet {
			t.Fatalf("coverage: only '%s' cover mode is currently supported (#3587), got '%s'", goCoverModeSet, coverMode)
		}
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
	fmt.Fprintf(w, "mode: %s\n", coverMode)
	cover := processCover()
	for name, blocks := range cover {
		for _, b := range blocks {
			var counts = b.counts
			if coverMode == goCoverModeSet && counts > 0 {
				counts = 1
			}
			fmt.Fprintf(w, "%s:%d.%d,%d.%d %d %d\n", name,
				b.startLine, b.startCol,
				b.endLine, b.endCol,
				b.stmts,
				counts,
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

func addScriptToCoverage(c *Contract) {
	// Any garbage may be passed to deployment methods, filter out useless contracts
	// to avoid misleading behaviour during coverage collection.
	if c.DebugInfo == nil || c.Hash.Equals(util.Uint160{}) {
		return
	}
	coverageLock.Lock()
	defer coverageLock.Unlock()
	if _, ok := rawCoverage[c.Hash]; !ok {
		rawCoverage[c.Hash] = &scriptRawCoverage{debugInfo: c.DebugInfo}
	}
}
