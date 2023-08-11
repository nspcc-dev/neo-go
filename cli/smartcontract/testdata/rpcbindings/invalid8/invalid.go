package invalid8

type SomeStruct struct {
	Field int
	// RPC binding generator will convert this field into exported, which matches
	// exactly the existing Field.
	field int
}

func Main() SomeStruct {
	s := SomeStruct{
		Field: 1,
		field: 2,
	}
	return s
}
