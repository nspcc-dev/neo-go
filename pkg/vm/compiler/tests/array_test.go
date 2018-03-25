package compiler

var arrayTestCases = []testCase{
	{
		"assign int array",
		`
		package foo
		func Main() []int {
			x := []int{1, 2, 3}
			return x
		}
		`,
		"52c56b53525153c16c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"assign string array",
		`
		package foo
		func Main() []string {
			x := []string{"foo", "bar", "foobar"}
			return x
		}
		`,
		"52c56b06666f6f6261720362617203666f6f53c16c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"array item assign",
		`
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			y := x[0]
			return y
		}
		`,
		"53c56b52510053c16c766b00527ac46c766b00c300c36c766b51527ac46203006c766b51c3616c7566",
	},
	{
		"array item return",
		`
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1]
		}
		`,
		"52c56b52510053c16c766b00527ac46203006c766b00c351c3616c7566",
	},
	{
		"array item in bin expr",
		`
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1] + 10
		}
		`,
		"52c56b52510053c16c766b00527ac46203006c766b00c351c35a93616c7566",
	},
	{
		"array item ident",
		`
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x]
		}
		`,
		"53c56b516c766b00527ac452510053c16c766b51527ac46203006c766b51c36c766b00c3c3616c7566",
	},
	{
		"array item index with binExpr",
		`
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x + 1]
		}
		`,
		"53c56b516c766b00527ac452510053c16c766b51527ac46203006c766b51c36c766b00c35193c3616c7566",
	},
	{
		"array item struct",
		`
		package foo

		type Bar struct {
			arr []int
		}

		func Main() int {
			b := Bar{
				arr: []int{0, 1, 2},
			}
			x := b.arr[2]
			return x + 2
		}
		`,
		"53c56b6151c66b52510053c16c766b00527ac46c6c766b00527ac46c766b00c300c352c36c766b51527ac46203006c766b51c35293616c7566",
	},
}
