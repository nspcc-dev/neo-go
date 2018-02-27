package compiler

var boolTestCases = []testCase{
	{
		"bool assign",
		`
		package foo
		func Main() bool {
			x := true
			return x
		}
		`,
		"52c56b516c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"bool compare",
		`
		package foo
		func Main() int {
			x := true
			if x {
				return 10
			}
			return 0
		}
		`,
		"54c56b516c766b00527ac46c766b00c3640b006203005a616c756662030000616c7566",
	},
	{
		"bool compare verbose",
		`
		package foo
		func Main() int {
			x := true
			if x == true {
				return 10
			}
			return 0
		}
		`,
		"54c56b516c766b00527ac46c766b00c3519c640b006203005a616c756662030000616c7566",
	},
	//	{
	//		"bool invert (unary expr)",
	//		`
	//		package foo
	//		func Main() int {
	//			x := true
	//			if !x {
	//				return 10
	//			}
	//			return 0
	//		}
	//		`,
	//		"54c56b516c766b00527ac46c766b00c3630b006203005a616c756662030000616c7566",
	//	},
}
