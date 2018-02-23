package compiler

var customTypeTestCases = []testCase{
	{
		"test custom type",
		`
		package foo

		type bar int
		type specialString string

		func Main() specialString {
			var x bar
			var str specialString
			x = 10
			str = "some short string"
			if x == 10 {
				return str
			}
			return "none"
		}
		`,
		"55c56b5a6c766b00527ac411736f6d652073686f727420737472696e676c766b51527ac46c766b00c35a9c640f006203006c766b51c3616c7566620300046e6f6e65616c7566",
	},
}
