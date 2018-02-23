package compiler

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
		"test function call with no assign",
		`
		package testcase
		func Main() int {
			getSomeInteger()
			getSomeInteger()
			return 0
		}

		func getSomeInteger() int {
			return 0
		}
		`,
		"53c56b616511007561650c007562030000616c756651c56b62030000616c7566",
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
	{
		"function call with arguments of interface type",
		`
		package testcase
		func Main() interface{} {
			x := getSomeInteger(10)
			return x
		}

		func getSomeInteger(x interface{}) interface{} {
			return x
		}
		`,
		"52c56b5a616516006c766b00527ac46203006c766b00c3616c756652c56b6c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"function call with multiple arguments",
		`
		package testcase
		func Main() int {
			x := addIntegers(2, 4)
			return x
		}

		func addIntegers(x int, y int) int {
			return x + y
		}
		`,
		"52c56b52547c616516006c766b00527ac46203006c766b00c3616c756653c56b6c766b00527ac46c766b51527ac46203006c766b00c36c766b51c393616c7566",
	},
	{
		"test Main arguments",
		`
		package foo
		func Main(operation string, args []interface{}) int {
			if operation == "mintTokens" {
				return 1
			} 
			return 0
		}
		`,
		"55c56b6c766b00527ac46c766b51527ac46c766b00c30a6d696e74546f6b656e739c640b0062030051616c756662030000616c7566",
	},
}
