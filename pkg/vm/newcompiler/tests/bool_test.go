package newcompiler_test

var boolTestCases = []testCase{
	{
		"bool assign",
		`
		package foo
		func Main() bool {
			x := true
			return x
		}
		`,
		"52c56b516c766b00527ac46203006c766b00c3616c7566",
	},
}
