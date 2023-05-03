package rpcbinding

import (
	"fmt"
	"sort"
	"strings"
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
	return {{if and (not .ItemTo) (eq .Unwrapper "Item")}}func (item stackitem.Item, err error) ({{ .ReturnType }}, error) {
		if err != nil {
			return nil, err
		}
		return {{addIndent (etTypeConverter .ExtendedReturn "item") "\t"}}
	} ( {{- end -}} {{if .ItemTo -}} itemTo{{ .ItemTo }}( {{- end -}}
			unwrap.{{.Unwrapper}}(c.invoker.Call(Hash, "{{ .NameABI }}"
		{{- range $arg := .Arguments -}}, {{.Name}}{{end -}} )) {{- if or .ItemTo (eq .Unwrapper "Item") -}} ) {{- end}}
	{{- else -}} (*result.Invoke, error) {
	c.invoker.Call(Hash, "{{ .NameABI }}"
		{{- range $arg := .Arguments -}}, {{.Name}}{{end}})
	{{- end}}
}
{{- if eq .Unwrapper "SessionIterator"}}

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

{{range $name, $typ := .NamedTypes}}
// {{toTypeName $name}} is a contract-specific {{$name}} type used by its methods.
type {{toTypeName $name}} struct {
{{- range $m := $typ.Fields}}
	{{.Field}} {{etTypeToStr .ExtendedType}}
{{- end}}
}
{{end -}}
{{if .HasReader}}// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
{{if or .IsNep11D .IsNep11ND}}	nep11.Invoker
{{else -}}
{{ if .IsNep17}}	nep17.Invoker
{{else if len .SafeMethods}}	Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error)
{{end -}}
{{if .HasIterator}}	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error)
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
	MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error)
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
{{end}}
{{- range $name, $typ := .NamedTypes}}
// itemTo{{toTypeName $name}} converts stack item into *{{toTypeName $name}}.
func itemTo{{toTypeName $name}}(item stackitem.Item, err error) (*{{toTypeName $name}}, error) {
	if err != nil {
		return nil, err
	}
	var res = new({{toTypeName $name}})
	err = res.FromStackItem(item)
	return res, err
}

// FromStackItem retrieves fields of {{toTypeName $name}} from the given stack item
// and returns an error if so.
func (res *{{toTypeName $name}}) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != {{len $typ.Fields}} {
		return errors.New("wrong number of structure elements")
	}

{{if len .Fields}}	var (
		index = -1
		err error
	)
{{- range $m := $typ.Fields}}
	index++
	res.{{.Field}}, err = {{etTypeConverter .ExtendedType "arr[index]"}}
	if err != nil {
		return fmt.Errorf("field {{.Field}}: %w", err)
	}
{{end}}
{{end}}
	return nil
}
{{end}}`

type (
	ContractTmpl struct {
		binding.ContractTmpl

		SafeMethods []SafeMethodTmpl
		NamedTypes  map[string]binding.ExtendedType

		IsNep11D  bool
		IsNep11ND bool
		IsNep17   bool

		HasReader   bool
		HasWriter   bool
		HasIterator bool
	}

	SafeMethodTmpl struct {
		binding.MethodTmpl
		Unwrapper      string
		ItemTo         string
		ExtendedReturn binding.ExtendedType
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
	ctr.NamedTypes = cfg.NamedTypes

	var srcTemplate = template.Must(template.New("generate").Funcs(template.FuncMap{
		"addIndent":       addIndent,
		"etTypeConverter": etTypeConverter,
		"etTypeToStr": func(et binding.ExtendedType) string {
			r, _ := extendedTypeToGo(et, ctr.NamedTypes)
			return r
		},
		"toTypeName": toTypeName,
		"cutPointer": cutPointer,
	}).Parse(srcTmpl))

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

func extendedTypeToGo(et binding.ExtendedType, named map[string]binding.ExtendedType) (string, string) {
	switch et.Base {
	case smartcontract.AnyType:
		return "any", ""
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
		if len(et.Name) > 0 {
			return "*" + toTypeName(et.Name), "github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
		} else if et.Value != nil {
			if et.Value.Base == smartcontract.PublicKeyType { // Special array wrapper.
				return "keys.PublicKeys", "github.com/nspcc-dev/neo-go/pkg/crypto/keys"
			}
			sub, pkg := extendedTypeToGo(*et.Value, named)
			return "[]" + sub, pkg
		}
		return "[]any", ""

	case smartcontract.MapType:
		kt, _ := extendedTypeToGo(binding.ExtendedType{Base: et.Key}, named)
		vt, _ := extendedTypeToGo(*et.Value, named)
		return "map[" + kt + "]" + vt, "github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	case smartcontract.InteropInterfaceType:
		return "any", ""
	case smartcontract.VoidType:
		return "", ""
	}
	panic("unreachable")
}

func etTypeConverter(et binding.ExtendedType, v string) string {
	switch et.Base {
	case smartcontract.AnyType:
		return v + ".Value(), nil"
	case smartcontract.BoolType:
		return v + ".TryBool()"
	case smartcontract.IntegerType:
		return v + ".TryInteger()"
	case smartcontract.ByteArrayType, smartcontract.SignatureType:
		return v + ".TryBytes()"
	case smartcontract.StringType:
		return `func (item stackitem.Item) (string, error) {
		b, err := item.TryBytes()
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", errors.New("not a UTF-8 string")
		}
		return string(b), nil
	} (` + v + `)`
	case smartcontract.Hash160Type:
		return `func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (` + v + `)`
	case smartcontract.Hash256Type:
		return `func (item stackitem.Item) (util.Uint256, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint256{}, err
		}
		u, err := util.Uint256DecodeBytesBE(b)
		if err != nil {
			return util.Uint256{}, err
		}
		return u, nil
	} (` + v + `)`
	case smartcontract.PublicKeyType:
		return `func (item stackitem.Item) (*keys.PublicKey, error) {
		b, err := item.TryBytes()
		if err != nil {
			return nil, err
		}
		k, err := keys.NewPublicKeyFromBytes(b, elliptic.P256())
		if err != nil {
			return nil, err
		}
		return k, nil
	} (` + v + `)`
	case smartcontract.ArrayType:
		if len(et.Name) > 0 {
			return "itemTo" + toTypeName(et.Name) + "(" + v + ", nil)"
		} else if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func (item stackitem.Item) (` + at + `, error) {
		arr, ok := item.Value().([]stackitem.Item)
		if !ok {
			return nil, errors.New("not an array")
		}
		res := make(` + at + `, len(arr))
		for i := range res {
			res[i], err = ` + addIndent(etTypeConverter(*et.Value, "arr[i]"), "\t\t") + `
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
		}
		return res, nil
	} (` + v + `)`
		}
		return etTypeConverter(binding.ExtendedType{
			Base: smartcontract.ArrayType,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)

	case smartcontract.MapType:
		at, _ := extendedTypeToGo(et, nil)
		return `func (item stackitem.Item) (` + at + `, error) {
		m, ok := item.Value().([]stackitem.MapElement)
		if !ok {
			return nil, fmt.Errorf("%s is not a map", item.Type().String())
		}
		res := make(` + at + `)
		for i := range m {
			k, err := ` + addIndent(etTypeConverter(binding.ExtendedType{Base: et.Key}, "m[i].Key"), "\t\t") + `
			if err != nil {
				return nil, fmt.Errorf("key %d: %w", i, err)
			}
			v, err := ` + addIndent(etTypeConverter(*et.Value, "m[i].Value"), "\t\t") + `
			if err != nil {
				return nil, fmt.Errorf("value %d: %w", i, err)
			}
			res[k] = v
		}
		return res, nil
	} (` + v + `)`
	case smartcontract.InteropInterfaceType:
		return "item.Value(), nil"
	case smartcontract.VoidType:
		return ""
	}
	panic("unreachable")
}

func scTypeToGo(name string, typ smartcontract.ParamType, cfg *binding.Config) (string, string) {
	et, ok := cfg.Types[name]
	if !ok {
		et = binding.ExtendedType{Base: typ}
	}
	return extendedTypeToGo(et, cfg.NamedTypes)
}

func scTemplateToRPC(cfg binding.Config, ctr ContractTmpl, imports map[string]struct{}) ContractTmpl {
	for i := range ctr.Imports {
		imports[ctr.Imports[i]] = struct{}{}
	}
	ctr.Hash = fmt.Sprintf("%#v", cfg.Hash)
	for i := 0; i < len(ctr.Methods); i++ {
		abim := cfg.Manifest.ABI.GetMethod(ctr.Methods[i].NameABI, len(ctr.Methods[i].Arguments))
		if abim.Safe {
			ctr.SafeMethods = append(ctr.SafeMethods, SafeMethodTmpl{MethodTmpl: ctr.Methods[i]})
			et, ok := cfg.Types[abim.Name]
			if ok {
				ctr.SafeMethods[len(ctr.SafeMethods)-1].ExtendedReturn = et
				if abim.ReturnType == smartcontract.ArrayType && len(et.Name) > 0 {
					ctr.SafeMethods[len(ctr.SafeMethods)-1].ItemTo = cutPointer(ctr.Methods[i].ReturnType)
				}
			}
			ctr.Methods = append(ctr.Methods[:i], ctr.Methods[i+1:]...)
			i--
		} else {
			ctr.Methods[i].Comment = fmt.Sprintf("creates a transaction invoking `%s` method of the contract.", ctr.Methods[i].NameABI)
			if ctr.Methods[i].ReturnType == "bool" {
				imports["github.com/nspcc-dev/neo-go/pkg/smartcontract"] = struct{}{}
			}
		}
	}
	for _, et := range cfg.NamedTypes {
		addETImports(et, ctr.NamedTypes, imports)
	}
	if len(cfg.NamedTypes) > 0 {
		imports["errors"] = struct{}{}
	}

	for i := range ctr.SafeMethods {
		switch ctr.SafeMethods[i].ReturnType {
		case "any":
			abim := cfg.Manifest.ABI.GetMethod(ctr.SafeMethods[i].NameABI, len(ctr.SafeMethods[i].Arguments))
			if abim.ReturnType == smartcontract.InteropInterfaceType {
				imports["github.com/google/uuid"] = struct{}{}
				imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
				imports["github.com/nspcc-dev/neo-go/pkg/neorpc/result"] = struct{}{}
				ctr.SafeMethods[i].ReturnType = "uuid.UUID, result.Iterator"
				ctr.SafeMethods[i].Unwrapper = "SessionIterator"
				ctr.HasIterator = true
			} else {
				imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
				ctr.SafeMethods[i].ReturnType = "any"
				ctr.SafeMethods[i].Unwrapper = "Item"
			}
		case "bool":
			ctr.SafeMethods[i].Unwrapper = "Bool"
		case "*big.Int":
			ctr.SafeMethods[i].Unwrapper = "BigInt"
		case "string":
			ctr.SafeMethods[i].Unwrapper = "UTF8String"
		case "util.Uint160":
			ctr.SafeMethods[i].Unwrapper = "Uint160"
		case "util.Uint256":
			ctr.SafeMethods[i].Unwrapper = "Uint256"
		case "*keys.PublicKey":
			ctr.SafeMethods[i].Unwrapper = "PublicKey"
		case "[]byte":
			ctr.SafeMethods[i].Unwrapper = "Bytes"
		case "[]any":
			imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
			ctr.SafeMethods[i].ReturnType = "[]stackitem.Item"
			ctr.SafeMethods[i].Unwrapper = "Array"
		case "*stackitem.Map":
			ctr.SafeMethods[i].Unwrapper = "Map"
		case "[]bool":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfBools"
		case "[]*big.Int":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfBigInts"
		case "[][]byte":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfBytes"
		case "[]string":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfUTF8Strings"
		case "[]util.Uint160":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfUint160"
		case "[]util.Uint256":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfUint256"
		case "keys.PublicKeys":
			ctr.SafeMethods[i].Unwrapper = "ArrayOfPublicKeys"
		default:
			addETImports(ctr.SafeMethods[i].ExtendedReturn, ctr.NamedTypes, imports)
			ctr.SafeMethods[i].Unwrapper = "Item"
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

func addETImports(et binding.ExtendedType, named map[string]binding.ExtendedType, imports map[string]struct{}) {
	_, pkg := extendedTypeToGo(et, named)
	if pkg != "" {
		imports[pkg] = struct{}{}
	}
	// Additional packages used during decoding.
	switch et.Base {
	case smartcontract.StringType:
		imports["unicode/utf8"] = struct{}{}
		imports["errors"] = struct{}{}
	case smartcontract.PublicKeyType:
		imports["crypto/elliptic"] = struct{}{}
	case smartcontract.MapType:
		imports["fmt"] = struct{}{}
	case smartcontract.ArrayType:
		imports["errors"] = struct{}{}
		imports["fmt"] = struct{}{}
	}
	if et.Value != nil {
		addETImports(*et.Value, named, imports)
	}
	if et.Base == smartcontract.MapType {
		addETImports(binding.ExtendedType{Base: et.Key}, named, imports)
	}
	for i := range et.Fields {
		addETImports(et.Fields[i].ExtendedType, named, imports)
	}
}

func cutPointer(s string) string {
	if s[0] == '*' {
		return s[1:]
	}
	return s
}

func toTypeName(s string) string {
	return strings.Map(func(c rune) rune {
		if c == '.' {
			return -1
		}
		return c
	}, strings.ToUpper(s[0:1])+s[1:])
}

func addIndent(str string, ind string) string {
	return strings.ReplaceAll(str, "\n", "\n"+ind)
}
