package compiler

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
	{
		"add assign",
		`
		package foo
		func Main() int {
			x := 4
			x += 8
			return x
		}
		`,
		"53c56b546c766b00527ac46c766b00c358936c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"sub assign",
		`
		package foo
		func Main() int {
			x := 4
			x -= 2
			return x
		}
		`,
		"53c56b546c766b00527ac46c766b00c352946c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"mul assign",
		`
		package foo
		func Main() int {
			x := 4
			x *= 2
			return x
		}
		`,
		"53c56b546c766b00527ac46c766b00c352956c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"div assign",
		`
		package foo
		func Main() int {
			x := 4
			x /= 2
			return x
		}
		`,
		"53c56b546c766b00527ac46c766b00c352966c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"add assign binary expr",
		`
		package foo
		func Main() int {
			x := 4
			x += 6 + 2
			return x
		}
		`,
		"53c56b546c766b00527ac46c766b00c358936c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"add assign binary expr ident",
		`
		package foo
		func Main() int {
			x := 4
			y := 5
			x += 6 + y
			return x
		}
		`,
		"54c56b546c766b00527ac4556c766b51527ac46c766b00c3566c766b51c393936c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"decl assign",
		`
		package foo
		func Main() int {
			var x int = 4
			return x
		}
		`,
		"52c56b546c766b00527ac46203006c766b00c3616c7566",
	},
}
