package newcompiler

import "log"

// A structScope holds the positions for it's fields. Struct fields have different
// positions then local variables in any scope.
type structScope struct {
	// identifier of the initialized struct in the program.
	name string

	// a mapping of field identifier and its position.
	fields map[string]int
}

func newStructScope() *structScope {
	return &structScope{
		fields: map[string]int{},
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
		log.Fatalf("could not resolve field name %s for struct %s", name, s.name)
	}
	return i
}
