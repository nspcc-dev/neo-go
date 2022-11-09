package rpcbinding

import (
	"fmt"
	"sort"
	"text/template"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest/standard"
)

const srcTmpl = `
{{- define "SAFEMETHOD" -}}
// {{.Name}} {{.Comment}}
func (c *ContractReader) {{.Name}}({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) {{if .ReturnType }}({{ .ReturnType }}, error) {
	return unwrap.{{.CallFlag}}(c.invoker.Call(Hash, "{{ .NameABI }}"{{/* CallFlag field is used for function name */}}
		{{- range $arg := .Arguments -}}, {{.Name}}{{end}}))
	{{- else -}} (*result.Invoke, error) {
	c.invoker.Call(Hash, "{{ .NameABI }}"
		{{- range $arg := .Arguments -}}, {{.Name}}{{end}})
	{{- end}}
}
{{- if eq .CallFlag "SessionIterator"}}

// {{.Name}}Expanded is similar to {{.Name}} (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) {{.Name}}Expanded({{range $index, $arg := .Arguments}}{{.Name}} {{.Type}}, {{end}}_numOfIteratorItems int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(Hash, "{{.NameABI}}", _numOfIteratorItems{{range $arg := .Arguments}}, {{.Name}}{{end}}))
}
{{- end -}}
{{- end -}}
{{- define "METHOD" -}}
{{- if eq .ReturnType "bool"}}func scriptFor{{.Name}}({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(Hash, "{{ .NameABI }}"{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}})
}

{{end}}// {{.Name}} {{.Comment}}
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) {{.Name}}({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) (util.Uint256, uint32, error) {
	{{if ne .ReturnType "bool"}}return c.actor.SendCall(Hash, "{{ .NameABI }}"
	{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}}){{else}}script, err := scriptFor{{.Name}}({{- range $index, $arg := .Arguments -}}{{- if ne $index 0}}, {{end}}{{.Name}}{{end}})
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return c.actor.SendRun(script){{end}}
}

// {{.Name}}Transaction {{.Comment}}
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) {{.Name}}Transaction({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) (*transaction.Transaction, error) {
	{{if ne .ReturnType "bool"}}return c.actor.MakeCall(Hash, "{{ .NameABI }}"
	{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}}){{else}}script, err := scriptFor{{.Name}}({{- range $index, $arg := .Arguments -}}{{- if ne $index 0}}, {{end}}{{.Name}}{{end}})
	if err != nil {
		return nil, err
	}
	return c.actor.MakeRun(script){{end}}
}

// {{.Name}}Unsigned {{.Comment}}
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) {{.Name}}Unsigned({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) (*transaction.Transaction, error) {
	{{if ne .ReturnType "bool"}}return c.actor.MakeUnsignedCall(Hash, "{{ .NameABI }}", nil
	{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}}){{else}}script, err := scriptFor{{.Name}}({{- range $index, $arg := .Arguments -}}{{- if ne $index 0}}, {{end}}{{.Name}}{{end}})
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedRun(script, nil){{end}}
}
{{- end -}}
// Package {{.PackageName}} contains RPC wrappers for {{.ContractName}} contract.
package {{.PackageName}}

import (
{{range $m := .Imports}}	"{{ $m }}"
{{end}})

// Hash contains contract hash.
var Hash = {{ .Hash }}

{{if .HasReader}}// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
{{if or .IsNep11D .IsNep11ND}}	nep11.Invoker
{{else -}}
{{ if .IsNep17}}	nep17.Invoker
{{else if len .SafeMethods}}	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
{{end -}}
{{if .HasIterator}}	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...interface{}) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
{{end -}}
{{end -}}
}

{{end -}}
{{if .HasWriter}}// Actor is used by Contract to call state-changing methods.
type Actor interface {
{{- if .HasReader}}
	Invoker
{{end}}
{{- if or .IsNep11D .IsNep11ND}}
	nep11.Actor
{{else if .IsNep17}}
	nep17.Actor
{{end}}
{{- if len .Methods}}
	MakeCall(contract util.Uint160, method string, params ...interface{}) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...interface{}) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...interface{}) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
{{end -}}
}

{{end -}}
{{if .HasReader}}// ContractReader implements safe contract methods.
type ContractReader struct {
	{{if .IsNep11D}}nep11.DivisibleReader
	{{end -}}
	{{if .IsNep11ND}}nep11.NonDivisibleReader
	{{end -}}
	{{if .IsNep17}}nep17.TokenReader
	{{end -}}
	invoker Invoker
}

{{end -}}
{{if .HasWriter}}// Contract implements all contract methods.
type Contract struct {
	{{if .HasReader}}ContractReader
	{{end -}}
	{{if .IsNep11D}}nep11.DivisibleWriter
	{{end -}}
	{{if .IsNep11ND}}nep11.BaseWriter
	{{end -}}
	{{if .IsNep17}}nep17.TokenWriter
	{{end -}}
	actor Actor
}

{{end -}}
{{if .HasReader}}// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{
		{{- if .IsNep11D}}*nep11.NewDivisibleReader(invoker, Hash), {{end}}
		{{- if .IsNep11ND}}*nep11.NewNonDivisibleReader(invoker, Hash), {{end}}
		{{- if .IsNep17}}*nep17.NewReader(invoker, Hash), {{end -}}
		invoker}
}

{{end -}}
{{if .HasWriter}}// New creates an instance of Contract using Hash and the given Actor.
func New(actor Actor) *Contract {
	{{if .IsNep11D}}var nep11dt = nep11.NewDivisible(actor, Hash)
	{{end -}}
	{{if .IsNep11ND}}var nep11ndt = nep11.NewNonDivisible(actor, Hash)
	{{end -}}
	{{if .IsNep17}}var nep17t = nep17.New(actor, Hash)
	{{end -}}
	return &Contract{
		{{- if .HasReader}}ContractReader{
		{{- if .IsNep11D}}nep11dt.DivisibleReader, {{end -}}
		{{- if .IsNep11ND}}nep11ndt.NonDivisibleReader, {{end -}}
		{{- if .IsNep17}}nep17t.TokenReader, {{end -}}
		actor}, {{end -}}
		{{- if .IsNep11D}}nep11dt.DivisibleWriter, {{end -}}
		{{- if .IsNep11ND}}nep11ndt.BaseWriter, {{end -}}
		{{- if .IsNep17}}nep17t.TokenWriter, {{end -}}
		actor}
}

{{end -}}
{{range $m := .SafeMethods}}
{{template "SAFEMETHOD" $m }}
{{end}}
{{- range $m := .Methods}}
{{template "METHOD" $m }}
{{end}}`

var srcTemplate = template.Must(template.New("generate").Parse(srcTmpl))

type (
	ContractTmpl struct {
		binding.ContractTmpl

		SafeMethods []binding.MethodTmpl

		IsNep11D  bool
		IsNep11ND bool
		IsNep17   bool

		HasReader   bool
		HasWriter   bool
		HasIterator bool
	}
)

// NewConfig initializes and returns a new config instance.
func NewConfig() binding.Config {
	return binding.NewConfig()
}

// Generate writes Go file containing smartcontract bindings to the `cfg.Output`.
// It doesn't check manifest from Config for validity, incorrect manifest can
// lead to unexpected results.
func Generate(cfg binding.Config) error {
	// Avoid changing *cfg.Manifest.
	mfst := *cfg.Manifest
	mfst.ABI.Methods = make([]manifest.Method, len(mfst.ABI.Methods))
	copy(mfst.ABI.Methods, cfg.Manifest.ABI.Methods)
	cfg.Manifest = &mfst

	var imports = make(map[string]struct{})
	var ctr ContractTmpl

	// Strip standard methods from NEP-XX packages.
	for _, std := range cfg.Manifest.SupportedStandards {
		if std == manifest.NEP11StandardName {
			imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"] = struct{}{}
			if standard.ComplyABI(cfg.Manifest, standard.Nep11Divisible) == nil {
				mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep11Divisible)
				ctr.IsNep11D = true
			} else if standard.ComplyABI(cfg.Manifest, standard.Nep11NonDivisible) == nil {
				mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep11NonDivisible)
				ctr.IsNep11ND = true
			}
			break // Can't be NEP-17 at the same time.
		}
		if std == manifest.NEP17StandardName && standard.ComplyABI(cfg.Manifest, standard.Nep17) == nil {
			mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep17)
			imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"] = struct{}{}
			ctr.IsNep17 = true
			break // Can't be NEP-11 at the same time.
		}
	}

	// OnNepXXPayment handlers normally can't be called directly.
	if standard.ComplyABI(cfg.Manifest, standard.Nep11Payable) == nil {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep11Payable)
	}
	if standard.ComplyABI(cfg.Manifest, standard.Nep17Payable) == nil {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep17Payable)
	}

	ctr.ContractTmpl = binding.TemplateFromManifest(cfg, scTypeToGo)
	ctr = scTemplateToRPC(cfg, ctr, imports)

	return srcTemplate.Execute(cfg.Output, ctr)
}

func dropManifestMethods(meths []manifest.Method, manifested []manifest.Method) []manifest.Method {
	for _, m := range manifested {
		for i := 0; i < len(meths); i++ {
			if meths[i].Name == m.Name && len(meths[i].Parameters) == len(m.Parameters) {
				meths = append(meths[:i], meths[i+1:]...)
				i--
			}
		}
	}
	return meths
}

func dropStdMethods(meths []manifest.Method, std *standard.Standard) []manifest.Method {
	meths = dropManifestMethods(meths, std.Manifest.ABI.Methods)
	if std.Optional != nil {
		meths = dropManifestMethods(meths, std.Optional)
	}
	if std.Base != nil {
		return dropStdMethods(meths, std.Base)
	}
	return meths
}

func scTypeToGo(name string, typ smartcontract.ParamType, overrides map[string]binding.Override) (string, string) {
	switch typ {
	case smartcontract.AnyType:
		return "interface{}", ""
	case smartcontract.BoolType:
		return "bool", ""
	case smartcontract.IntegerType:
		return "*big.Int", "math/big"
	case smartcontract.ByteArrayType:
		return "[]byte", ""
	case smartcontract.StringType:
		return "string", ""
	case smartcontract.Hash160Type:
		return "util.Uint160", "github.com/nspcc-dev/neo-go/pkg/util"
	case smartcontract.Hash256Type:
		return "util.Uint256", "github.com/nspcc-dev/neo-go/pkg/util"
	case smartcontract.PublicKeyType:
		return "*keys.PublicKey", "github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	case smartcontract.SignatureType:
		return "[]byte", ""
	case smartcontract.ArrayType:
		return "[]interface{}", ""
	case smartcontract.MapType:
		return "*stackitem.Map", "github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	case smartcontract.InteropInterfaceType:
		return "interface{}", ""
	case smartcontract.VoidType:
		return "", ""
	default:
		panic("unreachable")
	}
}

func scTemplateToRPC(cfg binding.Config, ctr ContractTmpl, imports map[string]struct{}) ContractTmpl {
	for i := range ctr.Imports {
		imports[ctr.Imports[i]] = struct{}{}
	}
	ctr.Hash = fmt.Sprintf("%#v", cfg.Hash)
	for i := 0; i < len(ctr.Methods); i++ {
		abim := cfg.Manifest.ABI.GetMethod(ctr.Methods[i].NameABI, len(ctr.Methods[i].Arguments))
		if abim.Safe {
			ctr.SafeMethods = append(ctr.SafeMethods, ctr.Methods[i])
			ctr.Methods = append(ctr.Methods[:i], ctr.Methods[i+1:]...)
			i--
		} else {
			ctr.Methods[i].Comment = fmt.Sprintf("creates a transaction invoking `%s` method of the contract.", ctr.Methods[i].NameABI)
			if ctr.Methods[i].ReturnType == "bool" {
				imports["github.com/nspcc-dev/neo-go/pkg/smartcontract"] = struct{}{}
			}
		}
	}
	// We're misusing CallFlag field for function name here.
	for i := range ctr.SafeMethods {
		switch ctr.SafeMethods[i].ReturnType {
		case "interface{}":
			abim := cfg.Manifest.ABI.GetMethod(ctr.SafeMethods[i].NameABI, len(ctr.SafeMethods[i].Arguments))
			if abim.ReturnType == smartcontract.InteropInterfaceType {
				imports["github.com/google/uuid"] = struct{}{}
				imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
				imports["github.com/nspcc-dev/neo-go/pkg/neorpc/result"] = struct{}{}
				ctr.SafeMethods[i].ReturnType = "uuid.UUID, result.Iterator"
				ctr.SafeMethods[i].CallFlag = "SessionIterator"
				ctr.HasIterator = true
			} else {
				imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
				ctr.SafeMethods[i].ReturnType = "stackitem.Item"
				ctr.SafeMethods[i].CallFlag = "Item"
			}
		case "bool":
			ctr.SafeMethods[i].CallFlag = "Bool"
		case "*big.Int":
			ctr.SafeMethods[i].CallFlag = "BigInt"
		case "string":
			ctr.SafeMethods[i].CallFlag = "UTF8String"
		case "util.Uint160":
			ctr.SafeMethods[i].CallFlag = "Uint160"
		case "util.Uint256":
			ctr.SafeMethods[i].CallFlag = "Uint256"
		case "*keys.PublicKey":
			ctr.SafeMethods[i].CallFlag = "PublicKey"
		case "[]byte":
			ctr.SafeMethods[i].CallFlag = "Bytes"
		case "[]interface{}":
			imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
			ctr.SafeMethods[i].ReturnType = "[]stackitem.Item"
			ctr.SafeMethods[i].CallFlag = "Array"
		case "*stackitem.Map":
			ctr.SafeMethods[i].CallFlag = "Map"
		}
	}

	imports["github.com/nspcc-dev/neo-go/pkg/util"] = struct{}{}
	if len(ctr.SafeMethods) > 0 {
		imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"] = struct{}{}
		if !(ctr.IsNep17 || ctr.IsNep11D || ctr.IsNep11ND) {
			imports["github.com/nspcc-dev/neo-go/pkg/neorpc/result"] = struct{}{}
		}
	}
	if len(ctr.Methods) > 0 {
		imports["github.com/nspcc-dev/neo-go/pkg/core/transaction"] = struct{}{}
	}
	if len(ctr.Methods) > 0 || ctr.IsNep17 || ctr.IsNep11D || ctr.IsNep11ND {
		ctr.HasWriter = true
	}
	if len(ctr.SafeMethods) > 0 || ctr.IsNep17 || ctr.IsNep11D || ctr.IsNep11ND {
		ctr.HasReader = true
	}
	ctr.Imports = ctr.Imports[:0]
	for imp := range imports {
		ctr.Imports = append(ctr.Imports, imp)
	}
	sort.Strings(ctr.Imports)
	return ctr
}
