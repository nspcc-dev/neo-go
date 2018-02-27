package compiler

var runtimeTestCases = []testCase{
	{
		"Notify test",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/smartcontract/runtime"

		func Main() bool {
			runtime.Notify("hello")
			return true
		}
		`,
		"52c56b0568656c6c6f6168124e656f2e52756e74696d652e4e6f746966796162030051616c756652c56b6c766b00527ac462030000616c7566",
	},
	{
		"Log test",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/smartcontract/runtime"

		func Main() bool {
			runtime.Log("hello you there!")
			return true
		}
		`,
		"52c56b1068656c6c6f20796f752074686572652161680f4e656f2e52756e74696d652e4c6f676162030051616c756652c56b6c766b00527ac462030000616c7566",
	},
	{
		"GetTime test",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/smartcontract/runtime"

		func Main() int {
			t := runtime.GetTime()
			return t
		}
		`,
		"52c56b6168134e656f2e52756e74696d652e47657454696d65616c766b00527ac46203006c766b00c3616c756651c56b62030000616c7566",
	},
	{
		"GetTrigger test",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/smartcontract/runtime"

		func Main() int {
			trigger := runtime.GetTrigger()
			if trigger == runtime.Application() {
				return 1
			}
			if trigger == runtime.Verification() {
				return 2
			}
			return 0
		}
		`,
		"56c56b6168164e656f2e52756e74696d652e47657454726967676572616c766b00527ac46c766b00c361652c009c640b0062030051616c75666c766b00c3616523009c640b0062030052616c756662030000616c756651c56b6203000110616c756651c56b6203000100616c756651c56b62030000616c7566",
	},
	{
		"check witness",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/smartcontract/runtime"

		func Main() int {
			owner := []byte{0xaf, 0x12, 0xa8, 0x68, 0x7b, 0x14, 0x94, 0x8b, 0xc4, 0xa0, 0x08, 0x12, 0x8a, 0x55, 0x0a, 0x63, 0x69, 0x5b, 0xc1, 0xa5}
			isOwner := runtime.CheckWitness(owner)
			if isOwner {
				return 1
			}
			return 0
		}
		`,
		"55c56b14af12a8687b14948bc4a008128a550a63695bc1a56c766b00527ac46c766b00c36168184e656f2e52756e74696d652e436865636b5769746e657373616c766b51527ac46c766b51c3640b0062030051616c756662030000616c756652c56b6c766b00527ac462030000616c7566",
	},
	{
		"getCurrentBlock",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/smartcontract/runtime"

		func Main() int {
			block := runtime.GetCurrentBlock()
			runtime.Notify(block)
			return 0
		}
		`,
		"53c56b61681b4e656f2e52756e74696d652e47657443757272656e74426c6f636b616c766b00527ac46c766b00c36168124e656f2e52756e74696d652e4e6f746966796162030000616c756651c56b62030000616c756652c56b6c766b00527ac462030000616c7566",
	},
}
