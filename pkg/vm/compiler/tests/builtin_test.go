package compiler

var builtinTestCases = []testCase{
	{
		"array len",
		`
		package foo

		func Main() int {
			x := []int{0, 1, 2}
			y := len(x)
			return y
		}
		`,
		"53c56b52510053c16c766b00527ac46c766b00c361c06c766b51527ac46203006c766b51c3616c7566",
	},
}
