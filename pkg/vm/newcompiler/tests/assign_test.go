package newcompiler_test

var assignTestCases = []testCase{
	{
		"chain define",
		`
		package foo
		func Main() int {
			x := 4
			y := x
			z := y
			foo := z
			bar := foo
			return bar
		}
		`,
		"56c56b546c766b00527ac46c766b00c36c766b51527ac46c766b51c36c766b52527ac46c766b52c36c766b53527ac46c766b53c36c766b54527ac46203006c766b54c3616c7566",
	},
	{
		"simple assign",
		`
		package foo
		func Main() int {
			x := 4
			x = 8
			return x
		}
		`,
		"53c56b546c766b00527ac4586c766b00527ac46203006c766b00c3616c7566",
	},
}
