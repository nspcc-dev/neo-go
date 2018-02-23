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
}
