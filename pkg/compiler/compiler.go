package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

const fileExt = "nef"

// Options contains all the parameters that affect the behaviour of the compiler.
type Options struct {
	// The extension of the output file default set to .nef
	Ext string

	// The name of the output file.
	Outfile string

	// The name of the output for debug info.
	DebugInfo string

	// The name of the output for contract manifest file.
	ManifestFile string

	// Contract features.
	ContractFeatures smartcontract.PropertyState

	// The list of standards supported by the contract.
	ContractSupportedStandards []string
}

type buildInfo struct {
	initialPackage string
	program        *loader.Program
}

// ForEachFile executes fn on each file used in current program.
func (c *codegen) ForEachFile(fn func(*ast.File, *types.Package)) {
	for i := range c.packages {
		pkg := c.buildInfo.program.Package(c.packages[i])
		c.typeInfo = &pkg.Info
		c.currPkg = pkg.Pkg
		for _, f := range pkg.Files {
			c.fillImportMap(f, pkg.Pkg)
			fn(f, pkg.Pkg)
		}
	}
}

// fillImportMap fills import map for f.
func (c *codegen) fillImportMap(f *ast.File, pkg *types.Package) {
	c.importMap = map[string]string{"": pkg.Path()}
	for _, imp := range f.Imports {
		// We need to load find package metadata because
		// name specified in `package ...` decl, can be in
		// conflict with package path.
		pkgPath := strings.Trim(imp.Path.Value, `"`)
		realPkg := c.buildInfo.program.Package(pkgPath)
		name := realPkg.Pkg.Name()
		if imp.Name != nil {
			name = imp.Name.Name
		}
		c.importMap[name] = realPkg.Pkg.Path()
	}
}

func getBuildInfo(src interface{}) (*buildInfo, error) {
	conf := loader.Config{ParserMode: parser.ParseComments}
	f, err := conf.ParseFile("", src)
	if err != nil {
		return nil, err
	}
	conf.CreateFromFiles("", f)

	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}

	return &buildInfo{
		initialPackage: f.Name.Name,
		program:        prog,
	}, nil
}

// Compile compiles a Go program into bytecode that can run on the NEO virtual machine.
func Compile(r io.Reader) ([]byte, error) {
	buf, _, err := CompileWithDebugInfo(r)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// CompileWithDebugInfo compiles a Go program into bytecode and emits debug info.
func CompileWithDebugInfo(r io.Reader) ([]byte, *DebugInfo, error) {
	ctx, err := getBuildInfo(r)
	if err != nil {
		return nil, nil, err
	}
	return CodeGen(ctx)
}

// CompileAndSave will compile and save the file to disk in the NEF format.
func CompileAndSave(src string, o *Options) ([]byte, error) {
	if !strings.HasSuffix(src, ".go") {
		return nil, fmt.Errorf("%s is not a Go file", src)
	}
	o.Outfile = strings.TrimSuffix(o.Outfile, fmt.Sprintf(".%s", fileExt))
	if len(o.Outfile) == 0 {
		o.Outfile = strings.TrimSuffix(src, ".go")
	}
	if len(o.Ext) == 0 {
		o.Ext = fileExt
	}
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return nil, err
	}
	b, di, err := CompileWithDebugInfo(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("error while trying to compile smart contract file: %v", err)
	}
	f, err := nef.NewFile(b)
	if err != nil {
		return nil, fmt.Errorf("error while trying to create .nef file: %v", err)
	}
	bytes, err := f.Bytes()
	if err != nil {
		return nil, fmt.Errorf("error while serializing .nef file: %v", err)
	}
	out := fmt.Sprintf("%s.%s", o.Outfile, o.Ext)
	err = ioutil.WriteFile(out, bytes, os.ModePerm)
	if err != nil {
		return b, err
	}
	if o.DebugInfo == "" && o.ManifestFile == "" {
		return b, nil
	}

	p, err := filepath.Abs(src)
	if err != nil {
		return b, err
	}
	di.Documents = append(di.Documents, p)

	if o.DebugInfo != "" {
		data, err := json.Marshal(di)
		if err != nil {
			return b, err
		}
		if err := ioutil.WriteFile(o.DebugInfo, data, os.ModePerm); err != nil {
			return b, err
		}
	}

	if o.ManifestFile != "" {
		m, err := di.ConvertToManifest(o.ContractFeatures, o.ContractSupportedStandards...)
		if err != nil {
			return b, errors.Wrap(err, "failed to convert debug info to manifest")
		}
		mData, err := json.Marshal(m)
		if err != nil {
			return b, errors.Wrap(err, "failed to marshal manifest")
		}
		return b, ioutil.WriteFile(o.ManifestFile, mData, os.ModePerm)
	}

	return b, nil
}
