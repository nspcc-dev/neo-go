package newcompiler_test

var functionCallTestCases = []testCase{
	{
		"simple function call",
		`
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			return x
		}
		`,
		"53c56b5a6c766b00527ac461651c006c766b51527ac46203006c766b00c36c766b51c393616c756652c56b5a6c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"multiple function calls",
		`
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			y := getSomeOtherInt()
			return x + y
		}

		func getSomeOtherInt() int {
			x := 8
			return x
		}
		`,
		"53c56b5a6c766b00527ac461651c006c766b51527ac46203006c766b00c36c766b51c393616c756653c56b5a6c766b00527ac461651c006c766b51527ac46203006c766b00c36c766b51c393616c756652c56b586c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"function call with arguments",
		`
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger(x)
			return y
		}

		func getSomeInteger(x int) int {
			y := 8
			return x + y
		}
		`,
		"53c56b5a6c766b00527ac46c766b00c3616516006c766b51527ac46203006c766b51c3616c756653c56b6c766b00527ac4586c766b51527ac46203006c766b00c36c766b51c393616c7566",
	},
}
