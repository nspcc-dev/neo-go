package compiler

var ifStatementTestCases = []testCase{
	{
		"if statement LT",
		`
		package testcase
		func Main() int {
			x := 10
			if x < 100 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c301649f640b0062030051616c756662030000616c7566",
	},
	{
		"if statement GT",
		`
		package testcase
		func Main() int {
			x := 10
			if x > 100 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c30164a0640b0062030051616c756662030000616c7566",
	},
	{
		"if statement GTE",
		`
		package testcase
		func Main() int {
			x := 10
			if x >= 100 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c30164a2640b0062030051616c756662030000616c7566",
	},
	{
		"complex if statement with LAND",
		`
		package testcase
		func Main() int {
			x := 10
			if x >= 10 && x <= 20 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c35aa26416006c766b00c30114a1640b0062030051616c756662030000616c7566",
	},
	{
		"complex if statement with LOR",
		`
		package testcase
		func Main() int {
			x := 10
			if x >= 10 || x <= 20 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c35aa2630e006c766b00c30114a1640b0062030051616c756662030000616c7566",
	},
	{
		"nested if statements",
		`
		package testcase
		func Main() int {
			x := 10
			if x > 10 {
				if x < 20 {
					return 1
				}
				return 2
			}
			return 0
		}
		`,
		"56c56b5a6c766b00527ac46c766b00c35aa0641e006c766b00c301149f640b0062030051616c756662030052616c756662030000616c7566",
	},
}
