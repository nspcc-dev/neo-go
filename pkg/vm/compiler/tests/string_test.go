package compiler

var stringTestCases = []testCase{
	{
		"simple string",
		`
		package testcase
		func Main() string {
			x := "NEO"
			return x
		}
		`,
		"52c56b034e454f6c766b00527ac46203006c766b00c3616c7566",
	},
}
