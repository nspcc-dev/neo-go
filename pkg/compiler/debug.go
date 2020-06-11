package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// DebugInfo represents smart-contract debug information.
type DebugInfo struct {
	EntryPoint string            `json:"entrypoint"`
	Documents  []string          `json:"documents"`
	Methods    []MethodDebugInfo `json:"methods"`
	Events     []EventDebugInfo  `json:"events"`
}

// MethodDebugInfo represents smart-contract's method debug information.
type MethodDebugInfo struct {
	ID string `json:"id"`
	// Name is the name of the method together with the namespace it belongs to.
	Name DebugMethodName `json:"name"`
	// Range is the range of smart-contract's opcodes corresponding to the method.
	Range DebugRange `json:"range"`
	// Parameters is a list of method's parameters.
	Parameters []DebugParam `json:"params"`
	// ReturnType is method's return type.
	ReturnType string   `json:"return-type"`
	Variables  []string `json:"variables"`
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
	// Name is a human-readable event name in a format "{namespace}-{name}".
	Name       string       `json:"name"`
	Parameters []DebugParam `json:"parameters"`
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
	Name string `json:"name"`
	Type string `json:"type"`
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

func (c *codegen) saveSequencePoint(n ast.Node) {
	if c.scope == nil {
		// do not save globals for now
		return
	}
	fset := c.buildInfo.program.Fset
	start := fset.Position(n.Pos())
	end := fset.Position(n.End())
	c.sequencePoints[c.scope.name] = append(c.sequencePoints[c.scope.name], DebugSeqPoint{
		Opcode:    c.prog.Len(),
		StartLine: start.Line,
		StartCol:  start.Offset,
		EndLine:   end.Line,
		EndCol:    end.Offset,
	})
}

func (c *codegen) emitDebugInfo() *DebugInfo {
	d := &DebugInfo{
		EntryPoint: mainIdent,
		Events:     []EventDebugInfo{},
	}
	for name, scope := range c.funcs {
		m := c.methodInfoFromScope(name, scope)
		if m.Range.Start == m.Range.End {
			continue
		}
		d.Methods = append(d.Methods, *m)
	}
	return d
}

func (c *codegen) registerDebugVariable(name string, expr ast.Expr) {
	if c.scope == nil {
		// do not save globals for now
		return
	}
	typ := c.scTypeFromExpr(expr)
	c.scope.variables = append(c.scope.variables, name+","+typ)
}

func (c *codegen) methodInfoFromScope(name string, scope *funcScope) *MethodDebugInfo {
	ps := scope.decl.Type.Params
	params := make([]DebugParam, 0, ps.NumFields())
	for i := range ps.List {
		for j := range ps.List[i].Names {
			params = append(params, DebugParam{
				Name: ps.List[i].Names[j].Name,
				Type: c.scTypeFromExpr(ps.List[i].Type),
			})
		}
	}
	return &MethodDebugInfo{
		ID:         name,
		Name:       DebugMethodName{Name: name},
		Range:      scope.rng,
		Parameters: params,
		ReturnType: c.scReturnTypeFromScope(scope),
		SeqPoints:  c.sequencePoints[name],
		Variables:  scope.variables,
	}
}

func (c *codegen) scReturnTypeFromScope(scope *funcScope) string {
	results := scope.decl.Type.Results
	switch results.NumFields() {
	case 0:
		return "Void"
	case 1:
		return c.scTypeFromExpr(results.List[0].Type)
	default:
		// multiple return values are not supported in debugger
		return "Any"
	}
}

func (c *codegen) scTypeFromExpr(typ ast.Expr) string {
	t := c.typeOf(typ)
	if c.typeOf(typ) == nil {
		return "Any"
	}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		info := t.Info()
		switch {
		case info&types.IsInteger != 0:
			return "Integer"
		case info&types.IsBoolean != 0:
			return "Boolean"
		case info&types.IsString != 0:
			return "String"
		default:
			return "Any"
		}
	case *types.Map:
		return "Map"
	case *types.Struct:
		return "Struct"
	case *types.Slice:
		if isByte(t.Elem()) {
			return "ByteArray"
		}
		return "Array"
	default:
		return "Any"
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

// convertToABI converts contract to the ABI struct for debugger.
// Note: manifest is taken from the external source, however it can be generated ad-hoc. See #1038.
func (di *DebugInfo) convertToABI(contract []byte, fs smartcontract.PropertyState) ABI {
	methods := make([]Method, 0)
	for _, method := range di.Methods {
		if method.Name.Name == di.EntryPoint {
			methods = append(methods, Method{
				Name:       method.Name.Name,
				Parameters: method.Parameters,
				ReturnType: method.ReturnType,
			})
			break
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
			HasStorage: fs&smartcontract.HasStorage != 0,
			IsPayable:  fs&smartcontract.IsPayable != 0,
		},
		EntryPoint: di.EntryPoint,
		Functions:  methods,
		Events:     events,
	}
}
