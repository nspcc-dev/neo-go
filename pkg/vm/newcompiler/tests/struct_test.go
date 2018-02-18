package newcompiler_test

var structTestCases = []testCase{
	{
		"struct field assign",
		`
		package foo
		type token struct {
			x int 
			y int
		}

		func Main() int {
			t := token {
				x: 2,
				y: 4,
			}

			age := t.x
			return age
		}
		`,
		"53c56b6152c66b526c766b00527ac4546c766b51527ac46c6c766b00527ac46c766b00c300c36c766b51527ac46203006c766b51c3616c7566",
	},
	{
		"struct field return",
		`
		package foo
		type token struct {
			x int 
			y int
		}

		func Main() int {
			t := token {
				x: 2,
				y: 4,
			}

			return t.x
		}
		`,
		"52c56b6152c66b526c766b00527ac4546c766b51527ac46c6c766b00527ac46203006c766b00c300c3616c7566",
	},
	{
		"complex struct",
		`
		package foo
		type token struct {
			x int 
			y int
		}

		func Main() int {
			x := 10

			t := token {
				x: 2,
				y: 4,
			}

			y := x + t.x

			return y
		}
		`,
		"54c56b5a6c766b00527ac46152c66b526c766b00527ac4546c766b51527ac46c6c766b51527ac46c766b00c36c766b51c300c3936c766b52527ac46203006c766b52c3616c7566",
	},
}
