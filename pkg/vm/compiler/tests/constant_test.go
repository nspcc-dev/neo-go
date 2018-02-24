package compiler

var constantTestCases = []testCase{
	{
		"basic constant",
		`
		package foo

		const x = 10

		func Main() int {
			return x + 2
		}
		`,
		// This ouput wil not be the same als the boa compiler.
		// The go compiler will automatically resolve binary expressions
		// involving constants.
		// x + 2 in this case will be resolved to 12.
		"52c56b5a6c766b00527ac46203005c616c7566",
	},
	{
		"shorthand multi const",
		`
		package foo

		const (
			z = 3
			y = 2
			x = 1
		)

		// should load al 3 constants in the Main.
		func Main() int {
			return 0
		}
		`,
		"54c56b536c766b00527ac4526c766b51527ac4516c766b52527ac462030000616c7566",
	},
	{
		"globals with function arguments",
		`
		package foobar

		const (
			bar = "FOO"
			foo = "BAR"
		)

		func something(x int) string {
			if x > 10 {
				return bar
			}
			return foo
		}

		func Main() string {
			trigger := 100
			x := something(trigger)
			return x
		}
		`,
		"55c56b03464f4f6c766b00527ac4034241526c766b51527ac401646c766b52527ac46c766b52c3616516006c766b53527ac46203006c766b53c3616c756656c56b6c766b00527ac403464f4f6c766b51527ac4034241526c766b52527ac46c766b00c35aa0640f006203006c766b51c3616c75666203006c766b52c3616c7566",
	},
}
