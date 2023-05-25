package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest/standard"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/rpcbinding"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
)

const fileExt = "nef"

// Options contains all the parameters that affect the behavior of the compiler.
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

	// GuessEventTypes specifies if types of runtime notifications need to be guessed
	// from the usage context. These types are used for RPC binding generation only and
	// can be defined for events with name known at the compilation time and without
	// variadic args usages. If some type is specified via config file, then the config's
	// one is preferable. Currently, event's parameter type is defined from the first
	// occurrence of event call.
	GuessEventTypes bool

	// Name is a contract's name to be written to manifest.
	Name string

	// SourceURL is a contract's source URL to be written to manifest.
	SourceURL string

	// Runtime notifications declared in the contract configuration file.
	ContractEvents []HybridEvent

	// DeclaredNamedTypes is the set of named types that were declared in the
	// contract configuration type and are the part of manifest events.
	DeclaredNamedTypes map[string]binding.ExtendedType

	// The list of standards supported by the contract.
	ContractSupportedStandards []string

	// SafeMethods contains a list of methods which will be marked as safe in manifest.
	SafeMethods []string

	// Overloads contains mapping from the compiled method name to the name emitted in manifest.
	// It can be used to provide method overloads as Go doesn't have such capability.
	Overloads map[string]string

	// Permissions is a list of permissions for every contract method.
	Permissions []manifest.Permission

	// BindingsFile contains configuration for smart-contract bindings generator.
	BindingsFile string
}

// HybridEvent represents the description of event emitted by the contract squashed
// with extended event's parameters description. We have it as a separate type for
// the user's convenience. It is applied for the smart contract configuration file
// only.
type HybridEvent struct {
	Name       string            `json:"name"`
	Parameters []HybridParameter `json:"parameters"`
}

// HybridParameter contains the manifest's event parameter description united with
// the extended type description for this parameter. It is applied for the smart
// contract configuration file only.
type HybridParameter struct {
	manifest.Parameter `yaml:",inline"`
	ExtendedType       *binding.ExtendedType `yaml:"extendedtype,omitempty"`
}

type buildInfo struct {
	config  *packages.Config
	program []*packages.Package
	options *Options
}

// ForEachPackage executes fn on each package used in the current program
// in the order they should be initialized.
func (c *codegen) ForEachPackage(fn func(*packages.Package)) {
	for _, pkgPath := range c.packages {
		p := c.packageCache[pkgPath]
		c.typeInfo = p.TypesInfo
		c.currPkg = p
		fn(p)
	}
}

// ForEachFile executes fn on each file used in the current program.
func (c *codegen) ForEachFile(fn func(*ast.File, *types.Package)) {
	c.ForEachPackage(func(pkg *packages.Package) {
		for _, f := range pkg.Syntax {
			c.fillImportMap(f, pkg)
			fn(f, pkg.Types)
		}
	})
}

// fillImportMap fills import map for f.
func (c *codegen) fillImportMap(f *ast.File, pkg *packages.Package) {
	c.importMap = map[string]string{"": pkg.PkgPath}
	for _, imp := range f.Imports {
		// We need to load find package metadata because
		// name specified in `package ...` decl, can be in
		// conflict with package path.
		pkgPath := strings.Trim(imp.Path.Value, `"`)
		realPkg := pkg.Imports[pkgPath]
		name := realPkg.Name
		if imp.Name != nil {
			name = imp.Name.Name
		}
		c.importMap[name] = realPkg.PkgPath
	}
}

func getBuildInfo(name string, src any) (*buildInfo, error) {
	dir, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	absName := dir

	singleFile := strings.HasSuffix(absName, ".go")
	if singleFile {
		dir = filepath.Dir(dir)
	}

	conf := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
		Fset:    token.NewFileSet(),
		Dir:     dir,
		Overlay: make(map[string][]byte),
	}

	var names []string
	if src != nil {
		var buf []byte
		var err error

		switch s := src.(type) {
		case string:
			buf = []byte(s)
		case io.Reader:
			buf, err = io.ReadAll(s)
			if err != nil {
				return nil, err
			}
		default:
			panic(fmt.Sprintf("unsupported src type: %T", s))
		}
		names = append(names, name)
		conf.Overlay[absName] = buf
	} else {
		if strings.HasSuffix(name, ".go") {
			names = append(names, "file="+absName)
		} else {
			names = append(names, "pattern="+absName)
		}
	}

	conf.ParseFile = func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		// When compiling a single file we can or can not load other files from the same package.
		// Here we chose the latter which is consistent with `go run` behavior.
		// Other dependencies should still be processed.
		if singleFile && filepath.Dir(filename) == filepath.Dir(absName) && filename != absName {
			return nil, nil
		}
		const mode = parser.AllErrors
		return parser.ParseFile(fset, filename, src, mode)
	}
	prog, err := packages.Load(conf, names...)
	if err != nil {
		return nil, err
	}
	for _, p := range prog {
		if len(p.Errors) != 0 {
			return nil, p.Errors[0]
		}
	}
	return &buildInfo{
		config:  conf,
		program: prog,
	}, nil
}

// Compile compiles a Go program into a bytecode that can run on the Neo virtual machine.
// If `r != nil`, `name` is interpreted as a filename, and `r` as file contents.
// Otherwise `name` is either a file name or a name of the directory containing source files.
func Compile(name string, r io.Reader) ([]byte, error) {
	f, _, err := CompileWithOptions(name, r, nil)
	if err != nil {
		return nil, err
	}

	return f.Script, nil
}

// CompileWithOptions compiles a Go program into bytecode with the provided compiler options.
func CompileWithOptions(name string, r io.Reader, o *Options) (*nef.File, *DebugInfo, error) {
	ctx, err := getBuildInfo(name, r)
	if err != nil {
		return nil, nil, err
	}
	ctx.options = o
	return codeGen(ctx)
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
	f, di, err := CompileWithOptions(src, nil, o)
	if err != nil {
		return nil, fmt.Errorf("error while trying to compile smart contract file: %w", err)
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
	err = os.WriteFile(out, bytes, os.ModePerm)
	if err != nil {
		return f.Script, err
	}
	if o.DebugInfo == "" && o.ManifestFile == "" && o.BindingsFile == "" {
		return f.Script, nil
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
			return f.Script, err
		}
		if err := os.WriteFile(o.DebugInfo, data, os.ModePerm); err != nil {
			return f.Script, err
		}
	}

	if o.BindingsFile != "" {
		cfg := binding.NewConfig()
		cfg.Package = di.MainPkg
		for _, m := range di.Methods {
			if !m.IsExported {
				continue
			}
			for _, p := range m.Parameters {
				pname := m.Name.Name + "." + p.Name
				if p.RealType.TypeName != "" {
					cfg.Overrides[pname] = p.RealType
				}
				if p.ExtendedType != nil {
					cfg.Types[pname] = *p.ExtendedType
				}
			}
			if m.ReturnTypeReal.TypeName != "" {
				cfg.Overrides[m.Name.Name] = m.ReturnTypeReal
			}
			if m.ReturnTypeExtended != nil {
				cfg.Types[m.Name.Name] = *m.ReturnTypeExtended
			}
		}
		if len(di.NamedTypes) > 0 {
			cfg.NamedTypes = di.NamedTypes
		}
		for name, et := range o.DeclaredNamedTypes {
			// TODO: handle name conflict (it can happen due to invalid user input e.g.)
			cfg.NamedTypes[name] = et
		}
		for _, e := range o.ContractEvents {
			eStructName := rpcbinding.ToEventBindingName(e.Name)
			for _, p := range e.Parameters {
				pStructName := rpcbinding.ToParameterBindingName(p.Name)
				if p.ExtendedType != nil {
					pName := eStructName + "." + pStructName
					cfg.Types[pName] = *p.ExtendedType
				}
			}
		}
		if o.GuessEventTypes {
			if len(di.EmittedEvents) > 0 {
				for eventName, eventUsages := range di.EmittedEvents {
					var manifestEvent HybridEvent
					for _, e := range o.ContractEvents {
						if e.Name == eventName {
							manifestEvent = e
							break
						}
					}
					if len(manifestEvent.Name) == 0 {
						return nil, fmt.Errorf("inconsistent usages of event `%s`: not declared in the contract config", eventName)
					}
					exampleUsage := eventUsages[0]
					for _, usage := range eventUsages {
						if len(usage.Params) != len(manifestEvent.Parameters) {
							return nil, fmt.Errorf("inconsistent usages of event `%s` against config: number of params mismatch: %d vs %d", eventName, len(exampleUsage.Params), len(manifestEvent.Parameters))
						}
						for i, actual := range usage.Params {
							mParam := manifestEvent.Parameters[i]
							// TODO: see the TestCompile_GuessEventTypes, "SC parameter type mismatch" section,
							// do we want to compare with actual.RealType? The conversion code is emitted by the
							// compiler for it, so we expect the parameter to be of the proper type.
							if !(mParam.Type == smartcontract.AnyType || actual.TypeSC == mParam.Type) {
								return nil, fmt.Errorf("inconsistent usages of event `%s` against config: SC type of param #%d mismatch: %s vs %s", eventName, i, actual.TypeSC, mParam.Type)
							}
							expected := exampleUsage.Params[i]
							if !actual.ExtendedType.Equals(expected.ExtendedType) {
								return nil, fmt.Errorf("inconsistent usages of event `%s`: extended type of param #%d mismatch", eventName, i)
							}
						}
					}
					eBindingName := rpcbinding.ToEventBindingName(eventName)
					for typeName, extType := range exampleUsage.ExtTypes {
						if _, ok := cfg.NamedTypes[typeName]; !ok {
							cfg.NamedTypes[typeName] = extType
						}
					}

					for _, p := range exampleUsage.Params {
						pBindingName := rpcbinding.ToParameterBindingName(p.Name)
						pname := eBindingName + "." + pBindingName
						if p.RealType.TypeName != "" {
							if _, ok := cfg.Overrides[pname]; !ok {
								cfg.Overrides[pname] = p.RealType
							}
						}
						if p.ExtendedType != nil {
							if _, ok := cfg.Types[pname]; !ok {
								cfg.Types[pname] = *p.ExtendedType
							}
						}
					}
				}
			}
		}
		data, err := yaml.Marshal(&cfg)
		if err != nil {
			return nil, fmt.Errorf("can't marshal bindings configuration: %w", err)
		}
		err = os.WriteFile(o.BindingsFile, data, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("can't write bindings configuration: %w", err)
		}
	}

	if o.ManifestFile != "" {
		m, err := CreateManifest(di, o)
		if err != nil {
			return f.Script, err
		}
		mData, err := json.Marshal(m)
		if err != nil {
			return f.Script, fmt.Errorf("failed to marshal manifest to JSON: %w", err)
		}
		return f.Script, os.WriteFile(o.ManifestFile, mData, os.ModePerm)
	}

	return f.Script, nil
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
	err = m.IsValid(util.Uint160{}) // Check as much as possible without hash.
	if err != nil {
		return m, fmt.Errorf("manifest is invalid: %w", err)
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
			expected := m.ABI.GetEvent(name)
			if expected == nil {
				return nil, fmt.Errorf("event '%s' is emitted but not specified in manifest", name)
			}
			for _, emitted := range di.EmittedEvents[name] {
				if len(emitted.Params) != len(expected.Parameters) {
					return nil, fmt.Errorf("event '%s' should have %d parameters but has %d",
						name, len(expected.Parameters), len(emitted.Params))
				}
				for j := range expected.Parameters {
					if expected.Parameters[j].Type == smartcontract.AnyType {
						continue
					}
					expectedT := expected.Parameters[j].Type
					if emitted.Params[j].TypeSC != expectedT {
						return nil, fmt.Errorf("event '%s' should have '%s' as type of %d parameter, "+
							"got: %s", name, expectedT, j+1, emitted.Params[j].TypeSC)
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
