package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// DebugInfo represents smart-contract debug information.
type DebugInfo struct {
	MainPkg   string            `json:"-"`
	Documents []string          `json:"documents"`
	Methods   []MethodDebugInfo `json:"methods"`
	Events    []EventDebugInfo  `json:"events"`
	// EmittedEvents contains events occurring in code.
	EmittedEvents map[string][][]string `json:"-"`
}

// MethodDebugInfo represents smart-contract's method debug information.
type MethodDebugInfo struct {
	// ID is the actual name of the method.
	ID string `json:"id"`
	// Name is the name of the method with the first letter in a lowercase
	// together with the namespace it belongs to. We need to keep the first letter
	// lowercased to match manifest standards.
	Name DebugMethodName `json:"name"`
	// IsExported defines whether method is exported.
	IsExported bool `json:"-"`
	// IsFunction defines whether method has no receiver.
	IsFunction bool `json:"-"`
	// Range is the range of smart-contract's opcodes corresponding to the method.
	Range DebugRange `json:"range"`
	// Parameters is a list of method's parameters.
	Parameters []DebugParam `json:"params"`
	// ReturnType is method's return type.
	ReturnType string `json:"return"`
	// ReturnTypeSC is return type to use in manifest.
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

// DebugRange represents method's section in bytecode.
type DebugRange struct {
	Start uint16
	End   uint16
}

// DebugParam represents variables's name and type.
type DebugParam struct {
	Name   string                  `json:"name"`
	Type   string                  `json:"type"`
	TypeSC smartcontract.ParamType `json:"-"`
}

func (c *codegen) saveSequencePoint(n ast.Node) {
	name := "init"
	if c.scope != nil {
		name = c.scope.name
	}

	fset := c.buildInfo.program.Fset
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
		MainPkg:   c.mainPkg.Pkg.Name(),
		Events:    []EventDebugInfo{},
		Documents: c.documents,
	}
	if c.initEndOffset > 0 {
		d.Methods = append(d.Methods, MethodDebugInfo{
			ID: manifest.MethodInit,
			Name: DebugMethodName{
				Name:      manifest.MethodInit,
				Namespace: c.mainPkg.Pkg.Name(),
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
		})
	}
	if c.deployEndOffset >= 0 {
		d.Methods = append(d.Methods, MethodDebugInfo{
			ID: manifest.MethodDeploy,
			Name: DebugMethodName{
				Name:      manifest.MethodDeploy,
				Namespace: c.mainPkg.Pkg.Name(),
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
		})
	}
	for name, scope := range c.funcs {
		m := c.methodInfoFromScope(name, scope)
		if m.Range.Start == m.Range.End {
			continue
		}
		d.Methods = append(d.Methods, *m)
	}
	d.EmittedEvents = c.emittedEvents
	return d
}

func (c *codegen) registerDebugVariable(name string, expr ast.Expr) {
	if c.scope == nil {
		// do not save globals for now
		return
	}
	_, vt := c.scAndVMTypeFromExpr(expr)
	c.scope.variables = append(c.scope.variables, name+","+vt.String())
}

func (c *codegen) methodInfoFromScope(name string, scope *funcScope) *MethodDebugInfo {
	ps := scope.decl.Type.Params
	params := make([]DebugParam, 0, ps.NumFields())
	for i := range ps.List {
		for j := range ps.List[i].Names {
			st, vt := c.scAndVMTypeFromExpr(ps.List[i].Type)
			params = append(params, DebugParam{
				Name:   ps.List[i].Names[j].Name,
				Type:   vt.String(),
				TypeSC: st,
			})
		}
	}
	ss := strings.Split(name, ".")
	name = ss[len(ss)-1]
	r, n := utf8.DecodeRuneInString(name)
	st, vt := c.scAndVMReturnTypeFromScope(scope)
	return &MethodDebugInfo{
		ID: name,
		Name: DebugMethodName{
			Name:      string(unicode.ToLower(r)) + name[n:],
			Namespace: scope.pkg.Name(),
		},
		IsExported:   scope.decl.Name.IsExported(),
		IsFunction:   scope.decl.Recv == nil,
		Range:        scope.rng,
		Parameters:   params,
		ReturnType:   vt,
		ReturnTypeSC: st,
		SeqPoints:    c.sequencePoints[name],
		Variables:    scope.variables,
	}
}

func (c *codegen) scAndVMReturnTypeFromScope(scope *funcScope) (smartcontract.ParamType, string) {
	results := scope.decl.Type.Results
	switch results.NumFields() {
	case 0:
		return smartcontract.VoidType, "Void"
	case 1:
		st, vt := c.scAndVMTypeFromExpr(results.List[0].Type)
		return st, vt.String()
	default:
		// multiple return values are not supported in debugger
		return smartcontract.AnyType, "Any"
	}
}

func scAndVMInteropTypeFromExpr(named *types.Named) (smartcontract.ParamType, stackitem.Type) {
	name := named.Obj().Name()
	pkg := named.Obj().Pkg().Name()
	switch pkg {
	case "runtime", "contract":
		return smartcontract.ArrayType, stackitem.ArrayT // Block, Transaction, Contract
	case "interop":
		if name != "Interface" {
			switch name {
			case "Hash160":
				return smartcontract.Hash160Type, stackitem.ByteArrayT
			case "Hash256":
				return smartcontract.Hash256Type, stackitem.ByteArrayT
			case "PublicKey":
				return smartcontract.PublicKeyType, stackitem.ByteArrayT
			case "Signature":
				return smartcontract.SignatureType, stackitem.ByteArrayT
			}
		}
	}
	return smartcontract.InteropInterfaceType, stackitem.InteropT
}

func (c *codegen) scAndVMTypeFromExpr(typ ast.Expr) (smartcontract.ParamType, stackitem.Type) {
	t := c.typeOf(typ)
	if c.typeOf(typ) == nil {
		return smartcontract.AnyType, stackitem.AnyT
	}
	if named, ok := t.(*types.Named); ok {
		if isInteropPath(named.String()) {
			return scAndVMInteropTypeFromExpr(named)
		}
	}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		info := t.Info()
		switch {
		case info&types.IsInteger != 0:
			return smartcontract.IntegerType, stackitem.IntegerT
		case info&types.IsBoolean != 0:
			return smartcontract.BoolType, stackitem.BooleanT
		case info&types.IsString != 0:
			return smartcontract.StringType, stackitem.ByteArrayT
		default:
			return smartcontract.AnyType, stackitem.AnyT
		}
	case *types.Map:
		return smartcontract.MapType, stackitem.MapT
	case *types.Struct:
		return smartcontract.ArrayType, stackitem.StructT
	case *types.Slice:
		if isByte(t.Elem()) {
			return smartcontract.ByteArrayType, stackitem.ByteArrayT
		}
		return smartcontract.ArrayType, stackitem.ArrayT
	default:
		return smartcontract.AnyType, stackitem.AnyT
	}
}

// MarshalJSON implements json.Marshaler interface.
func (d *DebugRange) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(d.Start), 10) + `-` +
		strconv.FormatUint(uint64(d.End), 10) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
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

// MarshalJSON implements json.Marshaler interface.
func (d *DebugParam) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Name + `,` + d.Type + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (d *DebugParam) UnmarshalJSON(data []byte) error {
	startS, endS, err := parsePairJSON(data, ",")
	if err != nil {
		return err
	}

	d.Name = startS
	d.Type = endS

	return nil
}

// ToManifestParameter converts DebugParam to manifest.Parameter
func (d *DebugParam) ToManifestParameter() manifest.Parameter {
	return manifest.Parameter{
		Name: d.Name,
		Type: d.TypeSC,
	}
}

// ToManifestMethod converts MethodDebugInfo to manifest.Method
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

// MarshalJSON implements json.Marshaler interface.
func (d *DebugMethodName) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Namespace + `,` + d.Name + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (d *DebugMethodName) UnmarshalJSON(data []byte) error {
	startS, endS, err := parsePairJSON(data, ",")
	if err != nil {
		return err
	}

	d.Namespace = startS
	d.Name = endS

	return nil
}

// MarshalJSON implements json.Marshaler interface.
func (d *DebugSeqPoint) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%d[%d]%d:%d-%d:%d", d.Opcode, d.Document,
		d.StartLine, d.StartCol, d.EndLine, d.EndCol)
	return []byte(`"` + s + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
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

// ConvertToManifest converts contract to the manifest.Manifest struct for debugger.
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
	result.Permissions = []manifest.Permission{
		{
			Contract: manifest.PermissionDesc{
				Type: manifest.PermissionWildcard,
			},
			Methods: manifest.WildStrings{},
		},
	}
	return result, nil
}
