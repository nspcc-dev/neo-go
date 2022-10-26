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
{{- define "METHOD" -}}
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
{{- end -}}
// Package {{.PackageName}} contains RPC wrappers for {{.ContractName}} contract.
package {{.PackageName}}

import (
{{range $m := .Imports}}	"{{ $m }}"
{{end}})

// Hash contains contract hash.
var Hash = {{ .Hash }}

// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	{{if or .IsNep11D .IsNep11ND}}nep11.Invoker
	{{end -}}
	{{if .IsNep17}}nep17.Invoker
	{{end -}}
	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	{{if .IsNep11D}}nep11.DivisibleReader
	{{end -}}
	{{if .IsNep11ND}}nep11.NonDivisibleReader
	{{end -}}
	{{if .IsNep17}}nep17.TokenReader
	{{end -}}
	invoker Invoker
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{
		{{- if .IsNep11D}}*nep11.NewDivisibleReader(invoker, Hash), {{end}}
		{{- if .IsNep11ND}}*nep11.NewNonDivisibleReader(invoker, Hash), {{end}}
		{{- if .IsNep17}}*nep17.NewReader(invoker, Hash), {{end -}}
		invoker}
}

{{range $m := .Methods}}
{{template "METHOD" $m }}
{{end}}`

var srcTemplate = template.Must(template.New("generate").Parse(srcTmpl))

type (
	ContractTmpl struct {
		binding.ContractTmpl
		IsNep11D  bool
		IsNep11ND bool
		IsNep17   bool
	}
)

// NewConfig initializes and returns a new config instance.
func NewConfig() binding.Config {
	return binding.NewConfig()
}

// Generate writes Go file containing smartcontract bindings to the `cfg.Output`.
func Generate(cfg binding.Config) error {
	bctr, err := binding.TemplateFromManifest(cfg, scTypeToGo)
	if err != nil {
		return err
	}
	ctr := scTemplateToRPC(cfg, bctr)

	return srcTemplate.Execute(cfg.Output, ctr)
}

func dropManifestMethods(meths []binding.MethodTmpl, manifested []manifest.Method) []binding.MethodTmpl {
	for _, m := range manifested {
		for i := 0; i < len(meths); i++ {
			if meths[i].NameABI == m.Name && len(meths[i].Arguments) == len(m.Parameters) {
				meths = append(meths[:i], meths[i+1:]...)
				i--
			}
		}
	}
	return meths
}

func dropStdMethods(meths []binding.MethodTmpl, std *standard.Standard) []binding.MethodTmpl {
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

func scTemplateToRPC(cfg binding.Config, bctr binding.ContractTmpl) ContractTmpl {
	var imports = make(map[string]struct{})
	var ctr = ContractTmpl{ContractTmpl: bctr}
	for i := range ctr.Imports {
		imports[ctr.Imports[i]] = struct{}{}
	}
	ctr.Hash = fmt.Sprintf("%#v", cfg.Hash)
	for _, std := range cfg.Manifest.SupportedStandards {
		if std == manifest.NEP11StandardName {
			imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"] = struct{}{}
			if standard.ComplyABI(cfg.Manifest, standard.Nep11Divisible) == nil {
				ctr.Methods = dropStdMethods(ctr.Methods, standard.Nep11Divisible)
				ctr.IsNep11D = true
			} else if standard.ComplyABI(cfg.Manifest, standard.Nep11NonDivisible) == nil {
				ctr.Methods = dropStdMethods(ctr.Methods, standard.Nep11NonDivisible)
				ctr.IsNep11ND = true
			}
			break // Can't be NEP-17 at the same time.
		}
		if std == manifest.NEP17StandardName && standard.ComplyABI(cfg.Manifest, standard.Nep17) == nil {
			ctr.Methods = dropStdMethods(ctr.Methods, standard.Nep17)
			imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"] = struct{}{}
			ctr.IsNep17 = true
			break // Can't be NEP-11 at the same time.
		}
	}
	for i := 0; i < len(ctr.Methods); i++ {
		abim := cfg.Manifest.ABI.GetMethod(ctr.Methods[i].NameABI, len(ctr.Methods[i].Arguments))
		if !abim.Safe {
			ctr.Methods = append(ctr.Methods[:i], ctr.Methods[i+1:]...)
			i--
		}
	}
	// We're misusing CallFlag field for function name here.
	for i := range ctr.Methods {
		switch ctr.Methods[i].ReturnType {
		case "interface{}":
			imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
			ctr.Methods[i].ReturnType = "stackitem.Item"
			ctr.Methods[i].CallFlag = "Item"
		case "bool":
			ctr.Methods[i].CallFlag = "Bool"
		case "*big.Int":
			ctr.Methods[i].CallFlag = "BigInt"
		case "string":
			ctr.Methods[i].CallFlag = "UTF8String"
		case "util.Uint160":
			ctr.Methods[i].CallFlag = "Uint160"
		case "util.Uint256":
			ctr.Methods[i].CallFlag = "Uint256"
		case "*keys.PublicKey":
			ctr.Methods[i].CallFlag = "PublicKey"
		case "[]byte":
			ctr.Methods[i].CallFlag = "Bytes"
		case "[]interface{}":
			imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
			ctr.Methods[i].ReturnType = "[]stackitem.Item"
			ctr.Methods[i].CallFlag = "Array"
		case "*stackitem.Map":
			ctr.Methods[i].CallFlag = "Map"
		}
	}

	imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"] = struct{}{}
	imports["github.com/nspcc-dev/neo-go/pkg/neorpc/result"] = struct{}{}
	ctr.Imports = ctr.Imports[:0]
	for imp := range imports {
		ctr.Imports = append(ctr.Imports, imp)
	}
	sort.Strings(ctr.Imports)
	return ctr
}
