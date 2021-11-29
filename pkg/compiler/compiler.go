package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest/standard"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"golang.org/x/tools/go/loader" //nolint:staticcheck // SA1019: package golang.org/x/tools/go/loader is deprecated
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

	// NoEventsCheck specifies if events emitted by contract needs to be present in manifest.
	// This setting has effect only if manifest is emitted.
	NoEventsCheck bool

	// NoStandardCheck specifies if supported standards compliance needs to be checked.
	// This setting has effect only if manifest is emitted.
	NoStandardCheck bool

	// NoPermissionsCheck specifies if permissions in YAML config need to be checked
	// against invocations performed by the contract.
	// This setting has effect only if manifest is emitted.
	NoPermissionsCheck bool

	// Name is contract's name to be written to manifest.
	Name string

	// SourceURL is contract's source URL to be written to manifest.
	SourceURL string

	// Runtime notifications.
	ContractEvents []manifest.Event

	// The list of standards supported by the contract.
	ContractSupportedStandards []string

	// SafeMethods contains list of methods which will be marked as safe in manifest.
	SafeMethods []string

	// Overloads contains mapping from compiled method name to the name emitted in manifest.
	// It can be used to provide method overloads as Go doesn't have such capability.
	Overloads map[string]string

	// Permissions is a list of permissions for every contract method.
	Permissions []manifest.Permission
}

type buildInfo struct {
	initialPackage string
	program        *loader.Program
	options        *Options
}

// ForEachPackage executes fn on each package used in the current program
// in the order they should be initialized.
func (c *codegen) ForEachPackage(fn func(*loader.PackageInfo)) {
	for i := range c.packages {
		pkg := c.buildInfo.program.Package(c.packages[i])
		c.typeInfo = &pkg.Info
		c.currPkg = pkg.Pkg
		fn(pkg)
	}
}

// ForEachFile executes fn on each file used in current program.
func (c *codegen) ForEachFile(fn func(*ast.File, *types.Package)) {
	c.ForEachPackage(func(pkg *loader.PackageInfo) {
		for _, f := range pkg.Files {
			c.fillImportMap(f, pkg.Pkg)
			fn(f, pkg.Pkg)
		}
	})
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

func getBuildInfo(name string, src interface{}) (*buildInfo, error) {
	conf := loader.Config{ParserMode: parser.ParseComments}
	if src != nil {
		f, err := conf.ParseFile(name, src)
		if err != nil {
			return nil, err
		}
		conf.CreateFromFiles("", f)
	} else {
		var names []string
		if strings.HasSuffix(name, ".go") {
			names = append(names, name)
		} else {
			ds, err := ioutil.ReadDir(name)
			if err != nil {
				return nil, fmt.Errorf("'%s' is neither Go source nor a directory", name)
			}
			for i := range ds {
				if !ds[i].IsDir() && strings.HasSuffix(ds[i].Name(), ".go") {
					names = append(names, filepath.Join(name, ds[i].Name()))
				}
			}
		}
		if len(names) == 0 {
			return nil, errors.New("no files provided")
		}
		conf.CreateFromFilenames("", names...)
	}

	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}

	return &buildInfo{
		initialPackage: prog.InitialPackages()[0].Pkg.Name(),
		program:        prog,
	}, nil
}

// Compile compiles a Go program into bytecode that can run on the NEO virtual machine.
// If `r != nil`, `name` is interpreted as a filename, and `r` as file contents.
// Otherwise `name` is either file name or name of the directory containing source files.
func Compile(name string, r io.Reader) ([]byte, error) {
	buf, _, err := CompileWithDebugInfo(name, r)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// CompileWithDebugInfo compiles a Go program into bytecode and emits debug info.
func CompileWithDebugInfo(name string, r io.Reader) ([]byte, *DebugInfo, error) {
	return CompileWithOptions(name, r, &Options{
		NoEventsCheck: true,
	})
}

// CompileWithOptions compiles a Go program into bytecode with provided compiler options.
func CompileWithOptions(name string, r io.Reader, o *Options) ([]byte, *DebugInfo, error) {
	ctx, err := getBuildInfo(name, r)
	if err != nil {
		return nil, nil, err
	}
	ctx.options = o
	return CodeGen(ctx)
}

// CompileAndSave will compile and save the file to disk in the NEF format.
func CompileAndSave(src string, o *Options) ([]byte, error) {
	o.Outfile = strings.TrimSuffix(o.Outfile, fmt.Sprintf(".%s", fileExt))
	if len(o.Outfile) == 0 {
		if strings.HasSuffix(src, ".go") {
			o.Outfile = strings.TrimSuffix(src, ".go")
		} else {
			o.Outfile = "out"
		}
	}
	if len(o.Ext) == 0 {
		o.Ext = fileExt
	}
	b, di, err := CompileWithOptions(src, nil, o)
	if err != nil {
		return nil, fmt.Errorf("error while trying to compile smart contract file: %w", err)
	}
	f, err := nef.NewFile(b)
	if err != nil {
		return nil, fmt.Errorf("error while trying to create .nef file: %w", err)
	}
	if o.SourceURL != "" {
		if len(o.SourceURL) > nef.MaxSourceURLLength {
			return nil, errors.New("too long source URL")
		}
		f.Source = o.SourceURL
		f.Checksum = f.CalculateChecksum()
	}
	bytes, err := f.Bytes()
	if err != nil {
		return nil, fmt.Errorf("error while serializing .nef file: %w", err)
	}
	out := fmt.Sprintf("%s.%s", o.Outfile, o.Ext)
	err = ioutil.WriteFile(out, bytes, os.ModePerm)
	if err != nil {
		return b, err
	}
	if o.DebugInfo == "" && o.ManifestFile == "" {
		return b, nil
	}

	if o.DebugInfo != "" {
		di.Events = make([]EventDebugInfo, len(o.ContractEvents))
		for i, e := range o.ContractEvents {
			params := make([]DebugParam, len(e.Parameters))
			for j, p := range e.Parameters {
				params[j] = DebugParam{
					Name: p.Name,
					Type: p.Type.String(),
				}
			}
			di.Events[i] = EventDebugInfo{
				ID: e.Name,
				// DebugInfo event name should be at the format {namespace},{name}
				// but we don't provide namespace via .yml config
				Name:       "," + e.Name,
				Parameters: params,
			}
		}
		data, err := json.Marshal(di)
		if err != nil {
			return b, err
		}
		if err := ioutil.WriteFile(o.DebugInfo, data, os.ModePerm); err != nil {
			return b, err
		}
	}

	if o.ManifestFile != "" {
		m, err := CreateManifest(di, o)
		if err != nil {
			return b, err
		}
		mData, err := json.Marshal(m)
		if err != nil {
			return b, fmt.Errorf("failed to marshal manifest to JSON: %w", err)
		}
		return b, ioutil.WriteFile(o.ManifestFile, mData, os.ModePerm)
	}

	return b, nil
}

// CreateManifest creates manifest and checks that is is valid.
func CreateManifest(di *DebugInfo, o *Options) (*manifest.Manifest, error) {
	m, err := di.ConvertToManifest(o)
	if err != nil {
		return m, fmt.Errorf("failed to convert debug info to manifest: %w", err)
	}
	for _, name := range o.SafeMethods {
		if m.ABI.GetMethod(name, -1) == nil {
			return m, fmt.Errorf("method %s is marked as safe but missing from manifest", name)
		}
	}
	if !o.NoStandardCheck {
		if err := standard.CheckABI(m, o.ContractSupportedStandards...); err != nil {
			return m, err
		}
		if m.ABI.GetMethod(manifest.MethodOnNEP11Payment, -1) != nil {
			if err := standard.CheckABI(m, manifest.NEP11Payable); err != nil {
				return m, err
			}
		}
		if m.ABI.GetMethod(manifest.MethodOnNEP17Payment, -1) != nil {
			if err := standard.CheckABI(m, manifest.NEP17Payable); err != nil {
				return m, err
			}
		}
	}
	if !o.NoEventsCheck {
		for name := range di.EmittedEvents {
			ev := m.ABI.GetEvent(name)
			if ev == nil {
				return nil, fmt.Errorf("event '%s' is emitted but not specified in manifest", name)
			}
			argsList := di.EmittedEvents[name]
			for i := range argsList {
				if len(argsList[i]) != len(ev.Parameters) {
					return nil, fmt.Errorf("event '%s' should have %d parameters but has %d",
						name, len(ev.Parameters), len(argsList[i]))
				}
				for j := range ev.Parameters {
					expected := ev.Parameters[j].Type.String()
					if argsList[i][j] != expected {
						return nil, fmt.Errorf("event '%s' should have '%s' as type of %d parameter, "+
							"got: %s", name, expected, j+1, argsList[i][j])
					}
				}
			}
		}
	}

	if !o.NoPermissionsCheck {
		// We can't perform full check for 2 reasons:
		// 1. Contract hash may not be available at compile time.
		// 2. Permission may be specified for a group of contracts by public key.
		// Thus only basic checks are performed.

		for h, methods := range di.InvokedContracts {
			knownHash := !h.Equals(util.Uint160{})

		methodLoop:
			for _, m := range methods {
				for _, p := range o.Permissions {
					// Group or wildcard permission is ok to try.
					if knownHash && p.Contract.Type == manifest.PermissionHash && !p.Contract.Hash().Equals(h) {
						continue
					}

					if p.Methods.Contains(m) {
						continue methodLoop
					}
				}

				if knownHash {
					return nil, fmt.Errorf("method '%s' of contract %s is invoked but"+
						" corresponding permission is missing", m, h.StringLE())
				}
				return nil, fmt.Errorf("method '%s' is invoked but"+
					" corresponding permission is missing", m)
			}
		}
	}
	return m, nil
}
