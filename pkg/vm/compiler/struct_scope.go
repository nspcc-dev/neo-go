package compiler

import (
	"go/constant"
	"go/types"
	"log"
)

// A structScope holds the positions for it's fields. Struct fields have different
// positions then local variables in any scope.
type structScope struct {
	// A pointer to the underlying type.
	t *types.Struct

	// A mapping of fieldnames identifier and its position.
	fields map[string]int

	// A mapping of fieldnames and with type and value.
	// This will be populated in "initFields" to initialize all
	// structs fields to their zero value.
	// strings: "" (just a pushf 0x00)
	// int: 0
	// bool: false
	typeAndValues map[string]types.TypeAndValue
}

// newStructScope will create a new structScope with all fields initialized.
func newStructScope(t *types.Struct) *structScope {
	s := &structScope{
		fields:        map[string]int{},
		typeAndValues: make(map[string]types.TypeAndValue, t.NumFields()),
		t:             t,
	}
	s.initFields()
	return s
}

func (s *structScope) initFields() {
	var tv types.TypeAndValue
	for i := 0; i < s.t.NumFields(); i++ {
		f := s.t.Field(i)
		s.newField(f.Name())

		switch t := f.Type().(type) {
		case *types.Basic:
			switch t.Kind() {
			case types.Int:
				tv = types.TypeAndValue{
					Type:  t,
					Value: constant.MakeInt64(0),
				}
			case types.String:
				tv = types.TypeAndValue{
					Type:  t,
					Value: constant.MakeString(""),
				}
			case types.Bool, types.UntypedBool:
				tv = types.TypeAndValue{
					Type:  t,
					Value: constant.MakeBool(false),
				}
			default:
				log.Fatalf("could not initialize struct field %s to zero, type: %s", f.Name(), t)
			}
		}
		s.typeAndValues[f.Name()] = tv
	}
}

func (s *structScope) newField(name string) int {
	i := len(s.fields)
	s.fields[name] = i
	return i
}

func (s *structScope) loadField(name string) int {
	i, ok := s.fields[name]
	if !ok {
		log.Fatalf("could not resolve field %s for struct %v", name, s)
	}
	return i
}

func (s *structScope) initialize(t *types.Struct) {
	s.t = t
	for i := 0; i < t.NumFields(); i++ {
		s.newField(t.Field(i).Name())
	}
}
