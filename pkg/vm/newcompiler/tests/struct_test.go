package newcompiler_test

var structTestCases = []testCase{
	{
		"basic struct",
		`
		package foo
		type token struct {
			name string
			amount int
		}

		func Main() int {
			t := token {
				name: "foo",
				amount: 1000,
			}

			return t.amount
		}
		`,
		"52c56b6152c66b03666f6f6c766b00527ac402e8036c766b51527ac46c6c766b00527ac46203006c766b00c351c3616c7566",
	},
}
