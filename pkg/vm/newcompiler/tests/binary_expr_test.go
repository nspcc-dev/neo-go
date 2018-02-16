package newcompiler_test

var binaryExprTestCases = []testCase{
	{
		"simple add",
		`
		package testcase
		func Main() int {
			x := 2 + 2
			return x
		}
		`,
		"52c56b546c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple sub",
		`
		package testcase
		func Main() int {
			x := 2 - 2
			return x
		}
		`,
		"52c56b006c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple div",
		`
		package testcase
		func Main() int {
			x := 2 / 2
			return x
		}
		`,
		"52c56b516c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple mul",
		`
		package testcase
		func Main() int {
			x := 4 * 2
			return x
		}
		`,
		"52c56b586c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple binary expr in return",
		`
		package testcase
		func Main() int {
			x := 2
			return 2 + x
		}
		`,
		"52c56b526c766b00527ac4620300526c766b00c393616c7566",
	},
	{
		"complex binary expr",
		`
		package testcase
		func Main() int {
			x := 4
			y := 8
			z := x + 2 + 2 - 8
			return y * z
		}
		`,
		"54c56b546c766b00527ac4586c766b51527ac46c766b00c35293529358946c766b52527ac46203006c766b51c36c766b52c395616c7566",
	},
}
