package compiler

var importTestCases = []testCase{
	{
		"import function",
		`
	package somethingelse

 	import "github.com/CityOfZion/neo-go/pkg/vm/compiler/tests/foo"

	func Main() int {
		i := foo.NewBar()
		return i
	}
	`,
		"52c56b616516006c766b00527ac46203006c766b00c3616c756651c56b6203005a616c7566",
	},
	{
		"import test",
		`
	 	package somethingwedontcareabout

		import "github.com/CityOfZion/neo-go/pkg/vm/compiler/tests/bar"

	 	func Main() int {
			 b := bar.Bar{
				 X: 4,
			 }
			 return b.Y
	 	}
	 	`,
		"52c56b6154c66b546c766b00527ac4006c766b51527ac4006c766b52527ac4006c766b53527ac46c6c766b00527ac46203006c766b00c351c3616c7566",
	},
}
