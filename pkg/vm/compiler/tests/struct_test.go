package compiler_test

var structTestCases = []testCase{
	{
		"struct field assign",
		`
		package foo
		func Main() int {
			t := token {
				x: 2,
				y: 4,
			}

			age := t.x
			return age
		}

		type token struct {
			x int 
			y int
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
			t.x = 10
			return t.x
		}
		`,
		"53c56b6152c66b526c766b00527ac4546c766b51527ac46c6c766b00527ac45a6c766b00c3007bc46203006c766b00c300c3616c7566",
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
	{
		"initialize same struct twice",
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			t1 := token {
				x: 2,
				y: 4,
			}
			t2 := token {
				x: 2,
				y: 4,
			}
			return t1.x + t2.y
		}
		`,
		"53c56b6152c66b526c766b00527ac4546c766b51527ac46c6c766b00527ac46152c66b526c766b00527ac4546c766b51527ac46c6c766b51527ac46203006c766b00c300c36c766b51c351c393616c7566",
	},
	{
		"struct methods",
		`
		package foo
		type token struct {
			x int
		}

		func(t token) getInteger() int {
			return t.x
		}

		func Main() int {
			t := token {
				x: 4, 
			}
			someInt := t.getInteger()
			return someInt
		}
		`,
		"53c56b6151c66b546c766b00527ac46c6c766b00527ac46c766b00c3616516006c766b51527ac46203006c766b51c3616c756652c56b6c766b00527ac46203006c766b00c300c3616c7566",
	},
	{
		"struct methods with arguments",
		`
		package foo
		type token struct {
			x int
		}

		// Also tests if x conflicts with t.x
		func(t token) addIntegers(x int, y int) int {
			return t.x + x + y
		}

		func Main() int {
			t := token {
				x: 4, 
			}
			someInt := t.addIntegers(2, 4)
			return someInt
		}
		`,
		"53c56b6151c66b546c766b00527ac46c6c766b00527ac46c766b00c352545272616516006c766b51527ac46203006c766b51c3616c756654c56b6c766b00527ac46c766b51527ac46c766b52527ac46203006c766b00c300c36c766b51c3936c766b52c393616c7566",
	},
	{
		"initialize struct partially",
		`
		package foo
		type token struct {
			x int
			y int
			z string
			b bool
		}

		func Main() int {
			t := token {
				x: 4,
			}
			return t.y
		}
		`,
		"52c56b6154c66b546c766b00527ac4006c766b51527ac4006c766b52527ac4006c766b53527ac46c6c766b00527ac46203006c766b00c351c3616c7566",
	},
	{
		"test return struct from func",
		`
		package foo
		type token struct {
			x int
			y int
			z string
			b bool
		}

		func newToken() token {
			return token{
				x: 1,
				y: 2, 
				z: "hello",
				b: false,
			}
		}

		func Main() token {
			return newToken()
		}
		`,
		"51c56b62030061650700616c756651c56b6203006154c66b516c766b00527ac4526c766b51527ac40568656c6c6f6c766b52527ac4006c766b53527ac46c616c7566",
	},
}
