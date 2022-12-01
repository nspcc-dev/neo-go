package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// DebugInfo represents smart-contract debug information.
type DebugInfo struct {
	MainPkg   string            `json:"-"`
	Hash      util.Uint160      `json:"hash"`
	Documents []string          `json:"documents"`
	Methods   []MethodDebugInfo `json:"methods"`
	// NamedTypes are exported structured types that have some name (even
	// if the original structure doesn't) and a number of internal fields.
	NamedTypes map[string]binding.ExtendedType `json:"-"`
	Events     []EventDebugInfo                `json:"events"`
	// EmittedEvents contains events occurring in code.
	EmittedEvents map[string][][]string `json:"-"`
	// InvokedContracts contains foreign contract invocations.
	InvokedContracts map[util.Uint160][]string `json:"-"`
	// StaticVariables contains a list of static variable names and types.
	StaticVariables []string `json:"static-variables"`
}

// MethodDebugInfo represents smart-contract's method debug information.
type MethodDebugInfo struct {
	// ID is the actual name of the method.
	ID string `json:"id"`
	// Name is the name of the method with the first letter in a lowercase
	// together with the namespace it belongs to. We need to keep the first letter
	// lowercased to match manifest standards.
	Name DebugMethodName `json:"name"`
	// IsExported defines whether the method is exported.
	IsExported bool `json:"-"`
	// IsFunction defines whether the method has no receiver.
	IsFunction bool `json:"-"`
	// Range is the range of smart-contract's opcodes corresponding to the method.
	Range DebugRange `json:"range"`
	// Parameters is a list of the method's parameters.
	Parameters []DebugParam `json:"params"`
	// ReturnType is the method's return type.
	ReturnType string `json:"return"`
	// ReturnTypeReal is the method's return type as specified in Go code.
	ReturnTypeReal binding.Override `json:"-"`
	// ReturnTypeExtended is the method's return type with additional data.
	ReturnTypeExtended *binding.ExtendedType `json:"-"`
	// ReturnTypeSC is a return type to use in manifest.
	ReturnTypeSC smartcontract.ParamType `json:"-"`
	Variables    []string                `json:"variables"`
	// SeqPoints is a map between source lines and byte-code instruction offsets.
	SeqPoints []DebugSeqPoint `json:"sequence-points"`
}

// DebugMethodName is a combination of a namespace and name.
type DebugMethodName struct {
	Namespace string
	Name      string
}

// EventDebugInfo represents smart-contract's event debug information.
type EventDebugInfo struct {
	ID string `json:"id"`
	// Name is a human-readable event name in a format "{namespace},{name}".
	Name       string       `json:"name"`
	Parameters []DebugParam `json:"params"`
}

// DebugSeqPoint represents break-point for debugger.
type DebugSeqPoint struct {
	// Opcode is an opcode's address.
	Opcode int
	// Document is an index of file where sequence point occurs.
	Document int
	// StartLine is the first line of the break-pointed statement.
	StartLine int
	// StartCol is the first column of the break-pointed statement.
	StartCol int
	// EndLine is the last line of the break-pointed statement.
	EndLine int
	// EndCol is the last column of the break-pointed statement.
	EndCol int
}

// DebugRange represents the method's section in bytecode.
type DebugRange struct {
	Start uint16
	End   uint16
}

// DebugParam represents the variables's name and type.
type DebugParam struct {
	Name         string                  `json:"name"`
	Type         string                  `json:"type"`
	RealType     binding.Override        `json:"-"`
	ExtendedType *binding.ExtendedType   `json:"-"`
	TypeSC       smartcontract.ParamType `json:"-"`
}

func (c *codegen) saveSequencePoint(n ast.Node) {
	name := "init"
	if c.scope != nil {
		name = c.scope.name
	}

	fset := c.buildInfo.config.Fset
	start := fset.Position(n.Pos())
	end := fset.Position(n.End())
	c.sequencePoints[name] = append(c.sequencePoints[name], DebugSeqPoint{
		Opcode:    c.prog.Len(),
		Document:  c.docIndex[start.Filename],
		StartLine: start.Line,
		StartCol:  start.Offset,
		EndLine:   end.Line,
		EndCol:    end.Offset,
	})
}

func (c *codegen) emitDebugInfo(contract []byte) *DebugInfo {
	d := &DebugInfo{
		Hash:            hash.Hash160(contract),
		MainPkg:         c.mainPkg.Name,
		Events:          []EventDebugInfo{},
		Documents:       c.documents,
		StaticVariables: c.staticVariables,
	}
	if c.initEndOffset > 0 {
		d.Methods = append(d.Methods, MethodDebugInfo{
			ID: manifest.MethodInit,
			Name: DebugMethodName{
				Name:      manifest.MethodInit,
				Namespace: c.mainPkg.Name,
			},
			IsExported: true,
			IsFunction: true,
			Range: DebugRange{
				Start: 0,
				End:   uint16(c.initEndOffset),
			},
			ReturnType:   "Void",
			ReturnTypeSC: smartcontract.VoidType,
			SeqPoints:    c.sequencePoints["init"],
			Variables:    c.initVariables,
		})
	}
	if c.deployEndOffset >= 0 {
		d.Methods = append(d.Methods, MethodDebugInfo{
			ID: manifest.MethodDeploy,
			Name: DebugMethodName{
				Name:      manifest.MethodDeploy,
				Namespace: c.mainPkg.Name,
			},
			IsExported: true,
			IsFunction: true,
			Range: DebugRange{
				Start: uint16(c.initEndOffset + 1),
				End:   uint16(c.deployEndOffset),
			},
			Parameters: []DebugParam{
				{
					Name:   "data",
					Type:   "Any",
					TypeSC: smartcontract.AnyType,
				},
				{
					Name:   "isUpdate",
					Type:   "Boolean",
					TypeSC: smartcontract.BoolType,
				},
			},
			ReturnType:   "Void",
			ReturnTypeSC: smartcontract.VoidType,
			SeqPoints:    c.sequencePoints[manifest.MethodDeploy],
			Variables:    c.deployVariables,
		})
	}

	start := len(d.Methods)
	d.NamedTypes = make(map[string]binding.ExtendedType)
	for name, scope := range c.funcs {
		m := c.methodInfoFromScope(name, scope, d.NamedTypes)
		if m.Range.Start == m.Range.End {
			continue
		}
		d.Methods = append(d.Methods, *m)
	}
	sort.Slice(d.Methods[start:], func(i, j int) bool {
		return d.Methods[start+i].Name.Name < d.Methods[start+j].Name.Name
	})
	d.EmittedEvents = c.emittedEvents
	d.InvokedContracts = c.invokedContracts
	return d
}

func (c *codegen) registerDebugVariable(name string, expr ast.Expr) {
	_, vt, _, _ := c.scAndVMTypeFromExpr(expr, nil)
	if c.scope == nil {
		c.staticVariables = append(c.staticVariables, name+","+vt.String())
		return
	}
	c.scope.variables = append(c.scope.variables, name+","+vt.String())
}

func (c *codegen) methodInfoFromScope(name string, scope *funcScope, exts map[string]binding.ExtendedType) *MethodDebugInfo {
	ps := scope.decl.Type.Params
	params := make([]DebugParam, 0, ps.NumFields())
	for i := range ps.List {
		for j := range ps.List[i].Names {
			st, vt, rt, et := c.scAndVMTypeFromExpr(ps.List[i].Type, exts)
			params = append(params, DebugParam{
				Name:         ps.List[i].Names[j].Name,
				Type:         vt.String(),
				ExtendedType: et,
				RealType:     rt,
				TypeSC:       st,
			})
		}
	}
	ss := strings.Split(name, ".")
	name = ss[len(ss)-1]
	r, n := utf8.DecodeRuneInString(name)
	st, vt, rt, et := c.scAndVMReturnTypeFromScope(scope, exts)

	return &MethodDebugInfo{
		ID: name,
		Name: DebugMethodName{
			Name:      string(unicode.ToLower(r)) + name[n:],
			Namespace: scope.pkg.Name(),
		},
		IsExported:         scope.decl.Name.IsExported(),
		IsFunction:         scope.decl.Recv == nil,
		Range:              scope.rng,
		Parameters:         params,
		ReturnType:         vt,
		ReturnTypeExtended: et,
		ReturnTypeReal:     rt,
		ReturnTypeSC:       st,
		SeqPoints:          c.sequencePoints[name],
		Variables:          scope.variables,
	}
}

func (c *codegen) scAndVMReturnTypeFromScope(scope *funcScope, exts map[string]binding.ExtendedType) (smartcontract.ParamType, string, binding.Override, *binding.ExtendedType) {
	results := scope.decl.Type.Results
	switch results.NumFields() {
	case 0:
		return smartcontract.VoidType, "Void", binding.Override{}, nil
	case 1:
		st, vt, s, et := c.scAndVMTypeFromExpr(results.List[0].Type, exts)
		return st, vt.String(), s, et
	default:
		// multiple return values are not supported in debugger
		return smartcontract.AnyType, "Any", binding.Override{}, nil
	}
}

func scAndVMInteropTypeFromExpr(named *types.Named, isPointer bool) (smartcontract.ParamType, stackitem.Type, binding.Override, *binding.ExtendedType) {
	name := named.Obj().Name()
	pkg := named.Obj().Pkg().Name()
	switch pkg {
	case "ledger", "contract":
		// Block, Transaction, Contract.
		typeName := pkg + "." + name
		et := &binding.ExtendedType{Base: smartcontract.ArrayType, Name: typeName}
		if isPointer {
			typeName = "*" + typeName
		}
		return smartcontract.ArrayType, stackitem.ArrayT, binding.Override{
			Package:  named.Obj().Pkg().Path(),
			TypeName: typeName,
		}, et
	case "interop":
		if name != "Interface" {
			over := binding.Override{
				Package:  interopPrefix,
				TypeName: "interop." + name,
			}
			switch name {
			case "Hash160":
				return smartcontract.Hash160Type, stackitem.ByteArrayT, over, nil
			case "Hash256":
				return smartcontract.Hash256Type, stackitem.ByteArrayT, over, nil
			case "PublicKey":
				return smartcontract.PublicKeyType, stackitem.ByteArrayT, over, nil
			case "Signature":
				return smartcontract.SignatureType, stackitem.ByteArrayT, over, nil
			}
		}
	}
	return smartcontract.InteropInterfaceType,
		stackitem.InteropT,
		binding.Override{TypeName: "interface{}"},
		&binding.ExtendedType{Base: smartcontract.InteropInterfaceType, Interface: "iterator"} // Temporarily all interops are iterators.
}

func (c *codegen) scAndVMTypeFromExpr(typ ast.Expr, exts map[string]binding.ExtendedType) (smartcontract.ParamType, stackitem.Type, binding.Override, *binding.ExtendedType) {
	return c.scAndVMTypeFromType(c.typeOf(typ), exts)
}

func (c *codegen) scAndVMTypeFromType(t types.Type, exts map[string]binding.ExtendedType) (smartcontract.ParamType, stackitem.Type, binding.Override, *binding.ExtendedType) {
	if t == nil {
		return smartcontract.AnyType, stackitem.AnyT, binding.Override{TypeName: "interface{}"}, nil
	}

	var isPtr bool

	named, isNamed := t.(*types.Named)
	if !isNamed {
		var ptr *types.Pointer
		if ptr, isPtr = t.(*types.Pointer); isPtr {
			named, isNamed = ptr.Elem().(*types.Named)
		}
	}
	if isNamed {
		if isInteropPath(named.String()) {
			st, vt, over, et := scAndVMInteropTypeFromExpr(named, isPtr)
			if et != nil && et.Base == smartcontract.ArrayType && exts != nil && exts[et.Name].Name != et.Name {
				_ = c.genStructExtended(named.Underlying().(*types.Struct), et.Name, exts)
			}
			return st, vt, over, et
		}
	}

	var over binding.Override
	switch t := t.Underlying().(type) {
	case *types.Basic:
		info := t.Info()
		switch {
		case info&types.IsInteger != 0:
			over.TypeName = "int"
			return smartcontract.IntegerType, stackitem.IntegerT, over, nil
		case info&types.IsBoolean != 0:
			over.TypeName = "bool"
			return smartcontract.BoolType, stackitem.BooleanT, over, nil
		case info&types.IsString != 0:
			over.TypeName = "string"
			return smartcontract.StringType, stackitem.ByteArrayT, over, nil
		default:
			over.TypeName = "interface{}"
			return smartcontract.AnyType, stackitem.AnyT, over, nil
		}
	case *types.Map:
		et := &binding.ExtendedType{
			Base: smartcontract.MapType,
		}
		et.Key, _, _, _ = c.scAndVMTypeFromType(t.Key(), exts)
		vt, _, over, vet := c.scAndVMTypeFromType(t.Elem(), exts)
		et.Value = vet
		if et.Value == nil {
			et.Value = &binding.ExtendedType{Base: vt}
		}
		over.TypeName = "map[" + t.Key().String() + "]" + over.TypeName
		return smartcontract.MapType, stackitem.MapT, over, et
	case *types.Struct:
		if isNamed {
			over.Package = named.Obj().Pkg().Path()
			over.TypeName = named.Obj().Pkg().Name() + "." + named.Obj().Name()
			_ = c.genStructExtended(t, over.TypeName, exts)
		} else {
			name := "unnamed"
			if exts != nil {
				for exts[name].Name == name {
					name = name + "X"
				}
				_ = c.genStructExtended(t, name, exts)
			}
		}
		return smartcontract.ArrayType, stackitem.StructT, over,
			&binding.ExtendedType{ // Value-less, refer to exts.
				Base: smartcontract.ArrayType,
				Name: over.TypeName,
			}

	case *types.Slice:
		if isByte(t.Elem()) {
			over.TypeName = "[]byte"
			return smartcontract.ByteArrayType, stackitem.ByteArrayT, over, nil
		}
		et := &binding.ExtendedType{
			Base: smartcontract.ArrayType,
		}
		vt, _, over, vet := c.scAndVMTypeFromType(t.Elem(), exts)
		et.Value = vet
		if et.Value == nil {
			et.Value = &binding.ExtendedType{
				Base: vt,
			}
		}
		if over.TypeName != "" {
			over.TypeName = "[]" + over.TypeName
		}
		return smartcontract.ArrayType, stackitem.ArrayT, over, et
	default:
		over.TypeName = "interface{}"
		return smartcontract.AnyType, stackitem.AnyT, over, nil
	}
}

func (c *codegen) genStructExtended(t *types.Struct, name string, exts map[string]binding.ExtendedType) *binding.ExtendedType {
	var et *binding.ExtendedType
	if exts != nil {
		if exts[name].Name != name {
			et = &binding.ExtendedType{
				Base:   smartcontract.ArrayType,
				Name:   name,
				Fields: make([]binding.FieldExtendedType, t.NumFields()),
			}
			exts[name] = *et // Prefill to solve recursive structures.
			for i := range et.Fields {
				field := t.Field(i)
				ft, _, _, fet := c.scAndVMTypeFromType(field.Type(), exts)
				if fet == nil {
					et.Fields[i].ExtendedType.Base = ft
				} else {
					et.Fields[i].ExtendedType = *fet
				}
				et.Fields[i].Field = field.Name()
			}
			exts[name] = *et // Set real structure data.
		} else {
			et = new(binding.ExtendedType)
			*et = exts[name]
		}
	}
	return et
}

// MarshalJSON implements the json.Marshaler interface.
func (d *DebugRange) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(d.Start), 10) + `-` +
		strconv.FormatUint(uint64(d.End), 10) + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *DebugRange) UnmarshalJSON(data []byte) error {
	startS, endS, err := parsePairJSON(data, "-")
	if err != nil {
		return err
	}
	start, err := strconv.ParseUint(startS, 10, 16)
	if err != nil {
		return err
	}
	end, err := strconv.ParseUint(endS, 10, 16)
	if err != nil {
		return err
	}

	d.Start = uint16(start)
	d.End = uint16(end)

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d *DebugParam) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Name + `,` + d.Type + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *DebugParam) UnmarshalJSON(data []byte) error {
	startS, endS, err := parsePairJSON(data, ",")
	if err != nil {
		return err
	}

	d.Name = startS
	d.Type = endS

	return nil
}

// ToManifestParameter converts DebugParam to manifest.Parameter.
func (d *DebugParam) ToManifestParameter() manifest.Parameter {
	return manifest.Parameter{
		Name: d.Name,
		Type: d.TypeSC,
	}
}

// ToManifestMethod converts MethodDebugInfo to manifest.Method.
func (m *MethodDebugInfo) ToManifestMethod() manifest.Method {
	var (
		result manifest.Method
	)
	parameters := make([]manifest.Parameter, len(m.Parameters))
	for i, p := range m.Parameters {
		parameters[i] = p.ToManifestParameter()
	}
	result.Name = m.Name.Name
	result.Offset = int(m.Range.Start)
	result.Parameters = parameters
	result.ReturnType = m.ReturnTypeSC
	return result
}

// MarshalJSON implements the json.Marshaler interface.
func (d *DebugMethodName) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Namespace + `,` + d.Name + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *DebugMethodName) UnmarshalJSON(data []byte) error {
	startS, endS, err := parsePairJSON(data, ",")
	if err != nil {
		return err
	}

	d.Namespace = startS
	d.Name = endS

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d *DebugSeqPoint) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%d[%d]%d:%d-%d:%d", d.Opcode, d.Document,
		d.StartLine, d.StartCol, d.EndLine, d.EndCol)
	return []byte(`"` + s + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *DebugSeqPoint) UnmarshalJSON(data []byte) error {
	_, err := fmt.Sscanf(string(data), `"%d[%d]%d:%d-%d:%d"`,
		&d.Opcode, &d.Document, &d.StartLine, &d.StartCol, &d.EndLine, &d.EndCol)
	return err
}

func parsePairJSON(data []byte, sep string) (string, string, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", "", err
	}
	ss := strings.SplitN(s, sep, 2)
	if len(ss) != 2 {
		return "", "", errors.New("invalid range format")
	}
	return ss[0], ss[1], nil
}

// ConvertToManifest converts a contract to the manifest.Manifest struct for debugger.
// Note: manifest is taken from the external source, however it can be generated ad-hoc. See #1038.
func (di *DebugInfo) ConvertToManifest(o *Options) (*manifest.Manifest, error) {
	methods := make([]manifest.Method, 0)
	for _, method := range di.Methods {
		if method.IsExported && method.IsFunction && method.Name.Namespace == di.MainPkg {
			mMethod := method.ToManifestMethod()
			for i := range o.SafeMethods {
				if mMethod.Name == o.SafeMethods[i] {
					mMethod.Safe = true
					break
				}
			}
			methods = append(methods, mMethod)
		}
	}

	result := manifest.NewManifest(o.Name)
	if o.ContractSupportedStandards != nil {
		result.SupportedStandards = o.ContractSupportedStandards
	}
	result.ABI = manifest.ABI{
		Methods: methods,
		Events:  o.ContractEvents,
	}
	if result.ABI.Events == nil {
		result.ABI.Events = make([]manifest.Event, 0)
	}
	result.Permissions = o.Permissions
	for name, emitName := range o.Overloads {
		m := result.ABI.GetMethod(name, -1)
		if m == nil {
			return nil, fmt.Errorf("overload for method %s was provided but it wasn't found", name)
		}
		if result.ABI.GetMethod(emitName, -1) == nil {
			return nil, fmt.Errorf("overload with target method %s was provided but it wasn't found", emitName)
		}

		realM := result.ABI.GetMethod(emitName, len(m.Parameters))
		if realM != nil {
			return nil, fmt.Errorf("conflict overload for %s: "+
				"multiple methods with the same number of parameters", name)
		}
		m.Name = emitName
		// Check the resulting name against set of safe methods.
		for i := range o.SafeMethods {
			if m.Name == o.SafeMethods[i] {
				m.Safe = true
				break
			}
		}
	}
	return result, nil
}
