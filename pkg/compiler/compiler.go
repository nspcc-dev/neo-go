package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"golang.org/x/tools/go/loader"
)

const fileExt = "avm"

// Options contains all the parameters that affect the behaviour of the compiler.
type Options struct {
	// The extension of the output file default set to .avm
	Ext string

	// The name of the output file.
	Outfile string

	// The name of the output for debug info.
	DebugInfo string

	// The name of the output for application binary interface info.
	ABIInfo string

	// Contract metadata.
	ContractFeatures smartcontract.PropertyState
}

type buildInfo struct {
	initialPackage string
	program        *loader.Program
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

// CompileAndSave will compile and save the file to disk.
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
	out := fmt.Sprintf("%s.%s", o.Outfile, o.Ext)
	err = ioutil.WriteFile(out, b, os.ModePerm)
	if o.DebugInfo == "" {
		return b, err
	}
	p, err := filepath.Abs(src)
	if err != nil {
		return b, err
	}
	di.Documents = append(di.Documents, p)
	data, err := json.Marshal(di)
	if err != nil {
		return b, err
	}
	if err := ioutil.WriteFile(o.DebugInfo, data, os.ModePerm); err != nil {
		return b, err
	}
	if o.ABIInfo == "" {
		return b, err
	}
	abi := di.convertToABI(b, o.ContractFeatures)
	abiData, err := json.Marshal(abi)
	if err != nil {
		return b, err
	}
	return b, ioutil.WriteFile(o.ABIInfo, abiData, os.ModePerm)
}
