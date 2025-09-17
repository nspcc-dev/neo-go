package rpcbinding

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"text/template"
	"unicode"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest/standard"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// The set of constants containing parts of RPC binding template. Each block of code
// including template definition and var/type/method definitions contain new line at the
// start and ends with a new line. On adding new block of code to the template, please,
// ensure that this block has new line at the start and in the end of the block.
const (
	eventDefinition = `{{ define "EVENT" }}
// {{.Name}} represents "{{.ManifestName}}" event emitted by the contract.
type {{.Name}} struct {
	{{- range $index, $arg := .Parameters}}
	{{ upperFirst .Name}} {{.Type}}
	{{- end}}
}
{{ end }}`

	safemethodDefinition = `{{ define "SAFEMETHOD" }}
// {{.Name}} {{.Comment}}
func (c *ContractReader) {{.Name}}({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) {{if .ReturnType }}({{ .ReturnType }}, error) {
	return {{if and (not .ItemTo) (eq .Unwrapper "Item")}}func(item stackitem.Item, err error) ({{ .ReturnType }}, error) {
		if err != nil {
			return nil, err
		}
		return {{addIndent (etTypeConverter .ExtendedReturn "item") "\t"}}
	} ( {{- end -}} {{if .ItemTo -}} itemTo{{ .ItemTo }}( {{- end -}}
			unwrap.{{.Unwrapper}}(c.invoker.Call(c.hash, "{{ .NameABI }}"
		{{- range $arg := .Arguments -}}, {{.Name}}{{end -}} )) {{- if or .ItemTo (eq .Unwrapper "Item") -}} ) {{- end}}
	{{- else -}} (*result.Invoke, error) {
	c.invoker.Call(c.hash, "{{ .NameABI }}"
		{{- range $arg := .Arguments -}}, {{.Name}}{{end}})
	{{- end}}
}
{{ if eq .Unwrapper "SessionIterator" }}
// {{.Name}}Expanded is similar to {{.Name}} (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) {{.Name}}Expanded({{range $index, $arg := .Arguments}}{{.Name}} {{.Type}}, {{end}}_numOfIteratorItems int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(c.hash, "{{.NameABI}}", _numOfIteratorItems{{range $arg := .Arguments}}, {{.Name}}{{end}}))
}
{{ end }}{{ end }}`
	methodDefinition = `{{ define "METHOD" }}{{ if eq .ReturnType "bool"}}
func (c *Contract) scriptFor{{.Name}}({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(c.hash, "{{ .NameABI }}"{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}})
}
{{ end }}
// {{.Name}} {{.Comment}}
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) {{.Name}}({{range $index, $arg := .Arguments -}}
	{{- if ne $index 0}}, {{end}}
		{{- .Name}} {{.Type}}
	{{- end}}) (util.Uint256, uint32, error) {
	{{if ne .ReturnType "bool"}}return c.actor.SendCall(c.hash, "{{ .NameABI }}"
	{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}}){{else}}script, err := c.scriptFor{{.Name}}({{- range $index, $arg := .Arguments -}}{{- if ne $index 0}}, {{end}}{{.Name}}{{end}})
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
	{{if ne .ReturnType "bool"}}return c.actor.MakeCall(c.hash, "{{ .NameABI }}"
	{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}}){{else}}script, err := c.scriptFor{{.Name}}({{- range $index, $arg := .Arguments -}}{{- if ne $index 0}}, {{end}}{{.Name}}{{end}})
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
	{{if ne .ReturnType "bool"}}return c.actor.MakeUnsignedCall(c.hash, "{{ .NameABI }}", nil
	{{- range $index, $arg := .Arguments -}}, {{.Name}}{{end}}){{else}}script, err := c.scriptFor{{.Name}}({{- range $index, $arg := .Arguments -}}{{- if ne $index 0}}, {{end}}{{.Name}}{{end}})
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedRun(script, nil){{end}}
}
{{end}}`

	bindingDefinition = `// Code generated by neo-go contract generate-rpcwrapper --manifest <file.json> --out <file.go> [--hash <hash>] [--config <config>]; DO NOT EDIT.

// Package {{.PackageName}} contains RPC wrappers for {{.ContractName}} contract.
package {{.PackageName}}

import (
{{range $m := .Imports}}	"{{ $m }}"
{{end}})
{{if len .Hash}}
// Hash contains contract hash.
var Hash = {{ .Hash }}
{{end -}}
{{- range $index, $typ := .NamedTypes }}
// {{toTypeName $typ.Name}} is a contract-specific {{$typ.Name}} type used by its methods.
type {{toTypeName $typ.Name}} struct {
{{- range $m := $typ.Fields}}
	{{ upperFirst .Field}} {{etTypeToStr .ExtendedType}}
{{- end}}
}
{{end}}
{{- range $e := .CustomEvents }}{{template "EVENT" $e }}{{ end -}}
{{- if .HasReader}}
// Invoker is used by ContractReader to call various safe methods.
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
{{- if .HasWriter}}
// Actor is used by Contract to call state-changing methods.
type Actor interface {
{{- if .HasReader}}
	Invoker
{{end}}
{{- if or .IsNep11D .IsNep11ND}}
	nep11.Actor
{{else if .IsNep17}}
	nep17.Actor
{{else if .IsNep22}}
	nep22.Actor
{{else if .IsNep31}}
	nep31.Actor
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
{{- if .HasReader}}
// ContractReader implements safe contract methods.
type ContractReader struct {
	{{if .IsNep11D}}nep11.DivisibleReader
	{{end -}}
	{{if .IsNep11ND}}nep11.NonDivisibleReader
	{{end -}}
	{{if .IsNep17}}nep17.TokenReader
	{{end -}}
	{{if .IsNep24}}nep24.RoyaltyReader
	{{end -}}
	invoker Invoker
	hash util.Uint160
}
{{end -}}
{{- if .HasWriter}}
// Contract implements all contract methods.
type Contract struct {
	{{if .HasReader}}ContractReader
	{{end -}}
	{{if .IsNep11D}}nep11.DivisibleWriter
	{{end -}}
	{{if .IsNep11ND}}nep11.BaseWriter
	{{end -}}
	{{if .IsNep17}}nep17.TokenWriter
	{{end -}}
	{{if .IsNep22}}nep22.Contract
	{{end -}}
	{{if .IsNep31}}nep31.Contract
	{{end -}}
	actor Actor
	hash util.Uint160
}
{{end -}}
{{- if .HasReader}}
// NewReader creates an instance of ContractReader using {{if len .Hash -}}Hash{{- else -}}provided contract hash{{- end}} and the given Invoker.
func NewReader(invoker Invoker{{- if not (len .Hash) -}}, hash util.Uint160{{- end -}}) *ContractReader {
	{{if len .Hash -}}
	var hash = Hash
	{{end -}}
	return &ContractReader{
		{{- if .IsNep11D}}*nep11.NewDivisibleReader(invoker, hash), {{end}}
		{{- if .IsNep11ND}}*nep11.NewNonDivisibleReader(invoker, hash), {{end}}
		{{- if .IsNep17}}*nep17.NewReader(invoker, hash), {{end -}}
		{{- if .IsNep24}}*nep24.NewRoyaltyReader(invoker, hash), {{end -}}
		invoker, hash}
}
{{end -}}
{{- if .HasWriter}}
// New creates an instance of Contract using {{if len .Hash -}}Hash{{- else -}}provided contract hash{{- end}} and the given Actor.
func New(actor Actor{{- if not (len .Hash) -}}, hash util.Uint160{{- end -}}) *Contract {
	{{if len .Hash -}}
	var hash = Hash
	{{end -}}
	{{if .IsNep11D}}var nep11dt = nep11.NewDivisible(actor, hash)
	{{end -}}
	{{if .IsNep11ND}}var nep11ndt = nep11.NewNonDivisible(actor, hash)
	{{end -}}
	{{if .IsNep17}}var nep17t = nep17.New(actor, hash)
	{{end -}}
	{{if .IsNep24}}var nep24t = nep24.NewRoyaltyReader(actor, hash)
	{{end -}}
	return &Contract{
		{{- if .HasReader}}ContractReader{
		{{- if .IsNep11D}}nep11dt.DivisibleReader, {{end -}}
		{{- if .IsNep11ND}}nep11ndt.NonDivisibleReader, {{end -}}
		{{- if .IsNep17}}nep17t.TokenReader, {{end -}}
		{{- if .IsNep24}}*nep24t, {{end -}}
		actor, hash}, {{end -}}
		{{- if .IsNep11D}}nep11dt.DivisibleWriter, {{end -}}
		{{- if .IsNep11ND}}nep11ndt.BaseWriter, {{end -}}
		{{- if .IsNep17}}nep17t.TokenWriter, {{end -}}
		{{- if .IsNep22}}nep22.NewContract(actor, hash), {{end -}}
		{{- if .IsNep31}}nep31.NewContract(actor, hash), {{end -}}
		actor, hash}
}
{{end -}}
{{- range $m := .SafeMethods }}{{template "SAFEMETHOD" $m }}{{ end -}}
{{- range $m := .Methods -}}{{template "METHOD" $m }}{{ end -}}
{{- range $index, $typ := .NamedTypes }}
// itemTo{{toTypeName $typ.Name}} converts stack item into *{{toTypeName $typ.Name}}.
// NULL item is returned as nil pointer without error.
func itemTo{{toTypeName $typ.Name}}(item stackitem.Item, err error) (*{{toTypeName $typ.Name}}, error) {
	if err != nil {
		return nil, err
	}
	_, null := item.(stackitem.Null)
	if null {
		return nil, nil
	}
	var res = new({{toTypeName $typ.Name}})
	err = res.FromStackItem(item)
	return res, err
}

// Ensure *{{toTypeName $typ.Name}} is a proper [stackitem.Convertible].
var _ = stackitem.Convertible(&{{toTypeName $typ.Name}}{})

// Ensure *{{toTypeName $typ.Name}} is a proper [smartcontract.Convertible].
var _ = smartcontract.Convertible(&{{toTypeName $typ.Name}}{})

// FromStackItem retrieves fields of {{toTypeName $typ.Name}} from the given
// [stackitem.Item] or returns an error if it's not possible to do to so.
// It implements [stackitem.Convertible] interface.
func (res *{{toTypeName $typ.Name}}) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != {{len $typ.Fields}} {
		return errors.New("wrong number of structure elements")
	}
{{if len .Fields}}
	var (
		index = -1
		err   error
	)
{{- range $m := $typ.Fields}}
	index++
	res.{{ upperFirst .Field}}, err = {{etTypeConverter .ExtendedType "arr[index]"}}
	if err != nil {
		return fmt.Errorf("field {{ upperFirst .Field}}: %w", err)
	}
{{end}}
{{- end}}
	return nil
}

// ToStackItem creates [stackitem.Item] representing {{toTypeName $typ.Name}}.
// It implements [stackitem.Convertible] interface.
func (res *{{toTypeName $typ.Name}}) ToStackItem() (stackitem.Item, error) {
	if res == nil {
		return stackitem.Null{}, nil
	}

	var (
		err    error
		itm    stackitem.Item
		items = make([]stackitem.Item, 0, {{len .Fields}})
	)

{{- range $m := $typ.Fields}}
	itm, err = {{goTypeConverter .ExtendedType (print "res." (upperFirst .Field))}}
	if err != nil {
		return nil, fmt.Errorf("field {{ upperFirst .Field}}: %w", err)
	}
	items = append(items, itm)
{{end}}
	return stackitem.NewStruct(items), nil
}

// ToSCParameter creates [smartcontract.Parameter] representing {{toTypeName $typ.Name}}.
// It implements [smartcontract.Convertible] interface so that {{toTypeName $typ.Name}}
// could be used with invokers.
func (res *{{toTypeName $typ.Name}}) ToSCParameter() (smartcontract.Parameter, error) {
	if res == nil {
		return smartcontract.Parameter{Type: smartcontract.AnyType}, nil
	}

	var (
		err    error
		prm    smartcontract.Parameter
		prms = make([]smartcontract.Parameter, 0, {{len .Fields}})
	)

{{- range $m := $typ.Fields}}
	prm, err = {{scTypeConverter .ExtendedType (print "res." (upperFirst .Field))}}
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field {{ upperFirst .Field}}: %w", err)
	}
	prms = append(prms, prm)
{{end}}
	return smartcontract.Parameter{Type: smartcontract.ArrayType, Value: prms}, nil
}
{{ end -}}
{{- range $e := .CustomEvents }}
// {{$e.Name}}sFromApplicationLog retrieves a set of all emitted events
// with "{{$e.ManifestName}}" name from the provided [result.ApplicationLog].
func {{$e.Name}}sFromApplicationLog(log *result.ApplicationLog) ([]*{{$e.Name}}, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}

	var res []*{{$e.Name}}
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "{{$e.ManifestName}}" {
				continue
			}
			event := new({{$e.Name}})
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize {{$e.Name}} from stackitem (execution #%d, event #%d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
}

// FromStackItem converts provided [stackitem.Array] to {{$e.Name}} or
// returns an error if it's not possible to do to so.
func (e *{{$e.Name}}) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != {{len $e.Parameters}} {
		return errors.New("wrong number of structure elements")
	}

	{{if len $e.Parameters}}var (
		index = -1
		err   error
	)
	{{- range $p := $e.Parameters}}
	index++
	e.{{ upperFirst .Name}}, err = {{etTypeConverter .ExtType "arr[index]"}}
	if err != nil {
		return fmt.Errorf("field {{ upperFirst .Name}}: %w", err)
	}
{{end}}
{{- end}}
	return nil
}
{{end -}}`

	srcTmpl = bindingDefinition +
		eventDefinition +
		safemethodDefinition +
		methodDefinition
)

type (
	ContractTmpl struct {
		binding.ContractTmpl

		SafeMethods  []SafeMethodTmpl
		CustomEvents []CustomEventTemplate
		NamedTypes   []binding.ExtendedType

		IsNep11D       bool
		IsNep11ND      bool
		IsNep17        bool
		IsNep22        bool
		IsNep24        bool
		IsNep24Payable bool
		IsNep31        bool

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

	CustomEventTemplate struct {
		// Name is the event's name that will be used as the event structure name in
		// the resulting RPC binding. It is a valid go structure name and may differ
		// from ManifestName.
		Name string
		// ManifestName is the event's name declared in the contract manifest.
		// It may contain any UTF8 character.
		ManifestName string
		Parameters   []EventParamTmpl
	}

	EventParamTmpl struct {
		binding.ParamTmpl

		// ExtType holds the event parameter's type information provided by Manifest,
		// i.e. simple types only.
		ExtType binding.ExtendedType
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
	mfst.ABI.Methods = slices.Clone(mfst.ABI.Methods)
	cfg.Manifest = &mfst

	var (
		imports = make(map[string]struct{})
		ctr     ContractTmpl
		isNep30 bool
	)

	// Strip standard methods from NEP-XX packages.
	for _, std := range cfg.Manifest.SupportedStandards {
		switch std {
		case manifest.NEP11StandardName:
			imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"] = struct{}{}
			if standard.ComplyABI(cfg.Manifest, standard.Nep11Divisible) == nil {
				ctr.IsNep11D = true
			} else if standard.ComplyABI(cfg.Manifest, standard.Nep11NonDivisible) == nil {
				ctr.IsNep11ND = true
			}
		case manifest.NEP17StandardName:
			if standard.ComplyABI(cfg.Manifest, standard.Nep17) == nil {
				imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"] = struct{}{}
				ctr.IsNep17 = true
			}
		case manifest.NEP22StandardName:
			if standard.ComplyABI(cfg.Manifest, standard.Nep22) == nil {
				imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep22"] = struct{}{}
				ctr.IsNep22 = true
			}
		case manifest.NEP24StandardName:
			if standard.ComplyABI(cfg.Manifest, standard.Nep24) == nil {
				imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep24"] = struct{}{}
				ctr.IsNep24 = true
			}
		case manifest.NEP24Payable:
			if standard.ComplyABI(cfg.Manifest, standard.Nep24Payable) == nil {
				ctr.IsNep24Payable = true
			}
		case manifest.NEP30StandardName:
			if standard.ComplyABI(cfg.Manifest, standard.Nep30) == nil {
				isNep30 = true
			}
		case manifest.NEP31StandardName:
			if standard.ComplyABI(cfg.Manifest, standard.Nep31) == nil {
				imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/nep31"] = struct{}{}
				ctr.IsNep31 = true
			}
		}
	}

	if ctr.IsNep11D {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep11Divisible)
	}
	if ctr.IsNep11ND {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep11NonDivisible)
	}
	if ctr.IsNep11D || ctr.IsNep11ND {
		mfst.ABI.Events = dropStdEvents(mfst.ABI.Events, standard.Nep11Base)
	}
	if ctr.IsNep17 {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep17)
		mfst.ABI.Events = dropStdEvents(mfst.ABI.Events, standard.Nep17)
	}
	if ctr.IsNep22 {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep22)
	}
	if ctr.IsNep24 {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep24)
		cfg = dropNep24Types(cfg)
	}
	if ctr.IsNep24Payable {
		mfst.ABI.Events = dropStdEvents(mfst.ABI.Events, standard.Nep24Payable)
	}
	if isNep30 {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep30)
	}
	if ctr.IsNep31 {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep31)
	}

	// OnNepXXPayment handlers normally can't be called directly.
	if standard.ComplyABI(cfg.Manifest, standard.Nep26) == nil {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep26)
	}
	if standard.ComplyABI(cfg.Manifest, standard.Nep27) == nil {
		mfst.ABI.Methods = dropStdMethods(mfst.ABI.Methods, standard.Nep27)
	}

	ctr.ContractTmpl = binding.TemplateFromManifest(cfg, scTypeToGo)
	ctr = scTemplateToRPC(cfg, ctr, imports, scTypeToGo)
	ctr.NamedTypes = make([]binding.ExtendedType, 0, len(cfg.NamedTypes))
	for k := range cfg.NamedTypes {
		ctr.NamedTypes = append(ctr.NamedTypes, cfg.NamedTypes[k])
	}
	slices.SortFunc(ctr.NamedTypes, func(a, b binding.ExtendedType) int { return cmp.Compare(a.Name, b.Name) })

	// Check resulting named types and events don't have duplicating field names.
	for _, t := range ctr.NamedTypes {
		fDict := make(map[string]struct{})
		for _, n := range t.Fields {
			name := upperFirst(n.Field)
			if _, ok := fDict[name]; ok {
				return fmt.Errorf("named type `%s` has two fields with identical resulting binding name `%s`", t.Name, name)
			}
			fDict[name] = struct{}{}
		}
	}
	for _, e := range ctr.CustomEvents {
		fDict := make(map[string]struct{})
		for _, n := range e.Parameters {
			name := upperFirst(n.Name)
			if _, ok := fDict[name]; ok {
				return fmt.Errorf("event `%s` has two fields with identical resulting binding name `%s`", e.Name, name)
			}
			fDict[name] = struct{}{}
		}
	}

	var srcTemplate = template.Must(template.New("generate").Funcs(template.FuncMap{
		"addIndent":       addIndent,
		"etTypeConverter": etTypeConverter,
		"etTypeToStr": func(et binding.ExtendedType) string {
			r, _ := extendedTypeToGo(et, cfg.NamedTypes)
			return r
		},
		"goTypeConverter": goTypeConverter,
		"scTypeConverter": scTypeConverter,
		"toTypeName":      toTypeName,
		"cutPointer":      cutPointer,
		"upperFirst":      upperFirst,
	}).Parse(srcTmpl))

	return binding.FExecute(srcTemplate, cfg.Output, ctr)
}

func dropManifestMethods(meths []manifest.Method, manifested []manifest.Method) []manifest.Method {
	return slices.DeleteFunc(meths, func(m manifest.Method) bool {
		return slices.ContainsFunc(manifested, func(e manifest.Method) bool {
			return 0 == cmp.Or(
				cmp.Compare(m.Name, e.Name),
				func() int {
					if e.Parameters == nil {
						return 0
					}
					return cmp.Compare(len(m.Parameters), len(e.Parameters))
				}(),
			)
		})
	})
}

func dropManifestEvents(events []manifest.Event, manifested []manifest.Event) []manifest.Event {
	return slices.DeleteFunc(events, func(e manifest.Event) bool {
		return slices.ContainsFunc(manifested, func(v manifest.Event) bool {
			return 0 == cmp.Or(
				cmp.Compare(e.Name, v.Name),
				cmp.Compare(len(e.Parameters), len(v.Parameters)),
			)
		})
	})
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

func dropStdEvents(events []manifest.Event, std *standard.Standard) []manifest.Event {
	events = dropManifestEvents(events, std.Manifest.ABI.Events)
	if std.Base != nil {
		return dropStdEvents(events, std.Base)
	}
	return events
}

// dropNep24Types removes NamedTypes of NEP-24 from the config if they are used only from the methods of the standard.
func dropNep24Types(cfg binding.Config) binding.Config {
	var targetTypeName string
	// Find structure returned by standard.MethodRoyaltyInfo method
	// and remove it from binding.Config.NamedTypes as it will be imported from nep24 package.
	if royaltyInfo, ok := cfg.Types[standard.MethodRoyaltyInfo]; ok && royaltyInfo.Value != nil {
		returnType, exists := cfg.NamedTypes[royaltyInfo.Value.Name]
		if !exists || returnType.Fields == nil || len(returnType.Fields) != 2 ||
			returnType.Fields[0].ExtendedType.Base != smartcontract.Hash160Type ||
			returnType.Fields[1].ExtendedType.Base != smartcontract.IntegerType {
			return cfg
		}
		targetTypeName = royaltyInfo.Value.Name
	} else {
		return cfg
	}
	found := false
	for _, typeDef := range cfg.Types {
		if typeDef.Value != nil && typeDef.Value.Name == targetTypeName {
			if found {
				return cfg
			}
			found = true
		}
	}

	if found {
		delete(cfg.NamedTypes, targetTypeName)
	}
	return cfg
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
		var vt string
		if et.Value != nil {
			vt, _ = extendedTypeToGo(*et.Value, named)
		} else {
			vt = "any"
		}
		return "map[" + kt + "]" + vt, "github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	case smartcontract.InteropInterfaceType:
		return "any", ""
	case smartcontract.VoidType:
		return "", ""
	default:
		panic("unreachable")
	}
}

func etTypeConverter(et binding.ExtendedType, v string) string {
	switch et.Base {
	case smartcontract.AnyType:
		return v + ".Value(), error(nil)"
	case smartcontract.BoolType:
		return v + ".TryBool()"
	case smartcontract.IntegerType:
		return v + ".TryInteger()"
	case smartcontract.ByteArrayType, smartcontract.SignatureType:
		return v + ".TryBytes()"
	case smartcontract.StringType:
		return `func(item stackitem.Item) (string, error) {
		b, err := item.TryBytes()
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", errors.New("not a UTF-8 string")
		}
		return string(b), nil
	}(` + v + `)`
	case smartcontract.Hash160Type:
		return `func(item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	}(` + v + `)`
	case smartcontract.Hash256Type:
		return `func(item stackitem.Item) (util.Uint256, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint256{}, err
		}
		u, err := util.Uint256DecodeBytesBE(b)
		if err != nil {
			return util.Uint256{}, err
		}
		return u, nil
	}(` + v + `)`
	case smartcontract.PublicKeyType:
		return `func(item stackitem.Item) (*keys.PublicKey, error) {
		b, err := item.TryBytes()
		if err != nil {
			return nil, err
		}
		k, err := keys.NewPublicKeyFromBytes(b, elliptic.P256())
		if err != nil {
			return nil, err
		}
		return k, nil
	}(` + v + `)`
	case smartcontract.ArrayType:
		if len(et.Name) > 0 {
			return "itemTo" + toTypeName(et.Name) + "(" + v + ", nil)"
		} else if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func(item stackitem.Item) (` + at + `, error) {
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
	}(` + v + `)`
		}
		return etTypeConverter(binding.ExtendedType{
			Base: smartcontract.ArrayType,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)

	case smartcontract.MapType:
		if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func(item stackitem.Item) (` + at + `, error) {
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
	}(` + v + `)`
		}
		return etTypeConverter(binding.ExtendedType{
			Base: smartcontract.MapType,
			Key:  et.Key,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)
	case smartcontract.InteropInterfaceType:
		return "item.Value(), nil"
	case smartcontract.VoidType:
		return ""
	default:
		panic("unreachable")
	}
}

func goTypeConverter(et binding.ExtendedType, v string) string {
	switch et.Base {
	case smartcontract.AnyType:
		return "stackitem.TryMake(" + v + ")"
	case smartcontract.BoolType:
		return "stackitem.NewBool(" + v + "), error(nil)"
	case smartcontract.IntegerType:
		return "(*stackitem.BigInteger)(" + v + "), error(nil)"
	case smartcontract.ByteArrayType, smartcontract.SignatureType:
		return "stackitem.NewByteArray(" + v + "), error(nil)"
	case smartcontract.StringType:
		return "stackitem.NewByteArray([]byte(" + v + ")), error(nil)"
	case smartcontract.Hash160Type:
		return "stackitem.NewByteArray(" + v + ".BytesBE()), error(nil)"
	case smartcontract.Hash256Type:
		return "stackitem.NewByteArray(" + v + ".BytesBE()), error(nil)"
	case smartcontract.PublicKeyType:
		return "stackitem.NewByteArray(" + v + ".Bytes()), error(nil)"
	case smartcontract.ArrayType:
		if len(et.Name) > 0 {
			return v + ".ToStackItem()"
		} else if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func(in ` + at + `) (stackitem.Item, error) {
		if in == nil {
			return stackitem.Null{}, nil
		}

		var items = make([]stackitem.Item, 0, len(in))
		for i, v := range in {
			itm, err := ` + goTypeConverter(*et.Value, "v") + `
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
			items = append(items, itm)
		}
		return stackitem.NewArray(items), nil
	}(` + v + `)`
		}
		return goTypeConverter(binding.ExtendedType{
			Base: smartcontract.ArrayType,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)

	case smartcontract.MapType:
		if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func(in ` + at + `) (stackitem.Item, error) {
		if in == nil {
			return stackitem.Null{}, nil
		}

		var m = stackitem.NewMap()
		for k, v := range in {
			iKey, err := ` + goTypeConverter(binding.ExtendedType{Base: et.Key}, "k") + `
			if err != nil {
				return nil, fmt.Errorf("key %v: %w", k, err)
			}
			iVal, err := ` + goTypeConverter(*et.Value, "v") + `
			if err != nil {
				return nil, fmt.Errorf("key %v, wrong value: %w", k, err)
			}
			m.Add(iKey, iVal)
		}
		return m, nil
	}(` + v + `)`
		}

		return goTypeConverter(binding.ExtendedType{
			Base: smartcontract.MapType,
			Key:  et.Key,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)
	case smartcontract.InteropInterfaceType:
		return "stackitem.TryMake(" + v + ")"
	case smartcontract.VoidType:
		return "stackitem.TryMake(" + v + ")"
	default:
		panic("unreachable")
	}
}

func scTypeConverter(et binding.ExtendedType, v string) string {
	switch et.Base {
	case smartcontract.AnyType, smartcontract.BoolType, smartcontract.IntegerType,
		smartcontract.ByteArrayType, smartcontract.SignatureType, smartcontract.StringType,
		smartcontract.Hash160Type, smartcontract.Hash256Type, smartcontract.PublicKeyType,
		smartcontract.InteropInterfaceType, smartcontract.VoidType:
		return "smartcontract.NewParameterFromValue(" + v + ")"
	case smartcontract.ArrayType:
		if len(et.Name) > 0 {
			return v + ".ToSCParameter()"
		} else if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func(in ` + at + `) (smartcontract.Parameter, error) {
		if in == nil {
			return smartcontract.Parameter{Type: smartcontract.AnyType}, nil
		}

		var prms = make([]smartcontract.Parameter, 0, len(in))
		for i, v := range in {
			prm, err := ` + scTypeConverter(*et.Value, "v") + `
			if err != nil {
				return smartcontract.Parameter{}, fmt.Errorf("item %d: %w", i, err)
			}
			prms = append(prms, prm)
		}
		return smartcontract.Parameter{Type: smartcontract.ArrayType, Value: prms}, nil
	}(` + v + `)`
		}
		return scTypeConverter(binding.ExtendedType{
			Base: smartcontract.ArrayType,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)

	case smartcontract.MapType:
		if et.Value != nil {
			at, _ := extendedTypeToGo(et, nil)
			return `func(in ` + at + `) (smartcontract.Parameter, error) {
		if in == nil {
			return smartcontract.Parameter{Type: smartcontract.AnyType}, nil
		}

		var prms = make([]smartcontract.ParameterPair, 0, len(in))
		for k, v := range in {
			iKey, err := ` + scTypeConverter(binding.ExtendedType{Base: et.Key}, "k") + `
			if err != nil {
				return smartcontract.Parameter{}, fmt.Errorf("key %v: %w", k, err)
			}
			iVal, err := ` + scTypeConverter(*et.Value, "v") + `
			if err != nil {
				return smartcontract.Parameter{}, fmt.Errorf("key %v, wrong value: %w", k, err)
			}
			prms = append(prms, smartcontract.ParameterPair{Key: iKey, Value: iVal})
		}
		return smartcontract.Parameter{Type: smartcontract.MapType, Value: prms}, nil
	}(` + v + `)`
		}

		return goTypeConverter(binding.ExtendedType{
			Base: smartcontract.MapType,
			Key:  et.Key,
			Value: &binding.ExtendedType{
				Base: smartcontract.AnyType,
			},
		}, v)
	default:
		panic("unreachable")
	}
}

func scTypeToGo(name string, typ smartcontract.ParamType, cfg *binding.Config) (string, string) {
	et, ok := cfg.Types[name]
	if !ok {
		et = binding.ExtendedType{Base: typ}
	}
	return extendedTypeToGo(et, cfg.NamedTypes)
}

func scTemplateToRPC(cfg binding.Config, ctr ContractTmpl, imports map[string]struct{}, scTypeConverter func(string, smartcontract.ParamType, *binding.Config) (string, string)) ContractTmpl {
	for i := range ctr.Imports {
		imports[ctr.Imports[i]] = struct{}{}
	}
	if !cfg.Hash.Equals(util.Uint160{}) {
		ctr.Hash = fmt.Sprintf("%#v", cfg.Hash)
	}
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
			ctr.Methods = slices.Delete(ctr.Methods, i, i+1)
			i--
		} else {
			ctr.Methods[i].Comment = fmt.Sprintf("creates a transaction invoking `%s` method of the contract.", ctr.Methods[i].NameABI)
			if ctr.Methods[i].ReturnType == "bool" {
				imports["github.com/nspcc-dev/neo-go/pkg/smartcontract"] = struct{}{}
			}
		}
	}
	for _, et := range cfg.NamedTypes {
		addETImports(et, cfg.NamedTypes, imports)
	}
	if len(cfg.NamedTypes) > 0 {
		imports["errors"] = struct{}{}
		imports["github.com/nspcc-dev/neo-go/pkg/smartcontract"] = struct{}{}
	}
	for _, abiEvent := range cfg.Manifest.ABI.Events {
		eBindingName := ToEventBindingName(abiEvent.Name)
		eTmp := CustomEventTemplate{
			Name:         eBindingName,
			ManifestName: abiEvent.Name,
		}
		for i := range abiEvent.Parameters {
			pBindingName := ToParameterBindingName(abiEvent.Parameters[i].Name)
			fullPName := eBindingName + "." + pBindingName
			typeStr, pkg := scTypeConverter(fullPName, abiEvent.Parameters[i].Type, &cfg)
			if pkg != "" {
				imports[pkg] = struct{}{}
			}

			var (
				extType binding.ExtendedType
				ok      bool
			)
			if extType, ok = cfg.Types[fullPName]; !ok {
				extType = binding.ExtendedType{
					Base: abiEvent.Parameters[i].Type,
				}
				addETImports(extType, cfg.NamedTypes, imports)
			}
			eTmp.Parameters = append(eTmp.Parameters, EventParamTmpl{
				ParamTmpl: binding.ParamTmpl{
					Name: pBindingName,
					Type: typeStr,
				},
				ExtType: extType,
			})
		}
		ctr.CustomEvents = append(ctr.CustomEvents, eTmp)
	}

	if len(ctr.CustomEvents) > 0 {
		imports["github.com/nspcc-dev/neo-go/pkg/neorpc/result"] = struct{}{}
		imports["github.com/nspcc-dev/neo-go/pkg/vm/stackitem"] = struct{}{}
		imports["fmt"] = struct{}{}
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
			addETImports(ctr.SafeMethods[i].ExtendedReturn, cfg.NamedTypes, imports)
			ctr.SafeMethods[i].Unwrapper = "Item"
		}
	}

	imports["github.com/nspcc-dev/neo-go/pkg/util"] = struct{}{}
	if len(ctr.SafeMethods) > 0 {
		imports["github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"] = struct{}{}
		if !(ctr.IsNep17 || ctr.IsNep11D || ctr.IsNep11ND || ctr.IsNep24) {
			imports["github.com/nspcc-dev/neo-go/pkg/neorpc/result"] = struct{}{}
		}
	}
	if len(ctr.Methods) > 0 {
		imports["github.com/nspcc-dev/neo-go/pkg/core/transaction"] = struct{}{}
	}
	if len(ctr.Methods) > 0 || ctr.IsNep17 || ctr.IsNep11D || ctr.IsNep11ND || ctr.IsNep22 || ctr.IsNep31 {
		ctr.HasWriter = true
	}
	if len(ctr.SafeMethods) > 0 || ctr.IsNep17 || ctr.IsNep11D || ctr.IsNep11ND || ctr.IsNep24 {
		ctr.HasReader = true
	}
	ctr.Imports = ctr.Imports[:0]
	for imp := range imports {
		ctr.Imports = append(ctr.Imports, imp)
	}
	slices.Sort(ctr.Imports)
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
	default:
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
	}, upperFirst(s))
}

func addIndent(str string, ind string) string {
	return strings.ReplaceAll(str, "\n", "\n"+ind)
}

// ToEventBindingName converts event name specified in the contract manifest to
// a valid go exported event structure name.
func ToEventBindingName(eventName string) string {
	return toPascalCase(eventName) + "Event"
}

// ToParameterBindingName converts parameter name specified in the contract
// manifest to a valid go structure's exported field name.
func ToParameterBindingName(paramName string) string {
	return toPascalCase(paramName)
}

// toPascalCase removes all non-unicode characters from the provided string and
// converts it to pascal case using space as delimiter.
func toPascalCase(s string) string {
	var res string
	for w := range strings.SplitSeq(s, " ") { // TODO: use DecodeRuneInString instead.
		var word string
		for _, ch := range w {
			var ok bool
			if len(res) == 0 && len(word) == 0 {
				ok = unicode.IsLetter(ch)
			} else {
				ok = unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
			}
			if ok {
				word += string(ch)
			}
		}
		if len(word) > 0 {
			res += upperFirst(word)
		}
	}
	return res
}

func upperFirst(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}
