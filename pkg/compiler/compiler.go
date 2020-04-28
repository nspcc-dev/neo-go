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

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"golang.org/x/tools/go/loader"
)

const fileExt = "avm"
const abiExt = "abi.json"

// Options contains all the parameters that affect the behaviour of the compiler.
type Options struct {
	// The extension of the output file default set to .avm
	Ext string

	// The name of the output file.
	Outfile string

	// The name of the output for debug info.
	DebugInfo string
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

// ABI represents ABI contract info in compatible with NEO Blockchain Toolkit format
type ABI struct {
	Hash       util.Uint160 `json:"hash"`
	Metadata   Metadata     `json:"metadata"`
	EntryPoint string       `json:"entrypoint"`
	Functions  []Method     `json:"functions"`
	Events     []Event      `json:"events"`
}

// Metadata represents ABI contract metadata
type Metadata struct {
	Author               string `json:"author"`
	Email                string `json:"email"`
	Version              string `json:"version"`
	Title                string `json:"title"`
	Description          string `json:"description"`
	HasStorage           bool   `json:"has-storage"`
	HasDynamicInvocation bool   `json:"has-dynamic-invoke"`
	IsPayable            bool   `json:"is-payable"`
}

// Method represents ABI method's metadata.
type Method struct {
	Name       string       `json:"name"`
	Parameters []DebugParam `json:"parameters"`
	ReturnType string       `json:"returntype"`
}

// Event represents ABI event's metadata.
type Event struct {
	Name       string       `json:"name"`
	Parameters []DebugParam `json:"parameters"`
}

// CompileAndSave will compile and save the file to disk.
func CompileAndSave(src string, contractDetails *request.ContractDetails, o *Options) ([]byte, error) {
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
	if contractDetails == nil {
		return b, nil
	}
	abiOut := fmt.Sprintf("%s.%s", o.Outfile, abiExt)
	abi := convertToABI(b, di, contractDetails)
	abiData, err := json.Marshal(abi)
	if err != nil {
		return b, err
	}
	return b, ioutil.WriteFile(abiOut, abiData, os.ModePerm)
}

func convertToABI(contract []byte, di *DebugInfo, cd *request.ContractDetails) ABI {
	methods := make([]Method, len(di.Methods))
	for i, method := range di.Methods {
		methods[i] = Method{
			Name:       method.Name.Name,
			Parameters: method.Parameters,
			ReturnType: method.ReturnType,
		}
	}
	events := make([]Event, len(di.Events))
	for i, event := range di.Events {
		events[i] = Event{
			Name:       event.Name,
			Parameters: event.Parameters,
		}
	}
	return ABI{
		Hash: hash.Hash160(contract),
		Metadata: Metadata{
			Author:               cd.Author,
			Email:                cd.Email,
			Version:              cd.Version,
			Title:                cd.ProjectName,
			Description:          cd.Description,
			HasStorage:           cd.HasStorage,
			HasDynamicInvocation: cd.HasDynamicInvocation,
			IsPayable:            cd.IsPayable,
		},
		EntryPoint: di.EntryPoint,
		Functions:  methods,
		Events:     events,
	}
}
